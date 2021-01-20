package staging

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/twitchtv/twirp"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/queue"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

func (c *controller) collectResult(queue *s2hv1.Queue) error {
	// check deploy and test result
	if queue.Status.KubeZipLog == "" {
		logZip, err := c.createDeploymentZipLogs(queue)
		if err != nil {
			return err
		}

		queue.Status.KubeZipLog = logZip

		if err = c.updateQueue(queue); err != nil {
			return err
		}
	}

	if err := c.setDeploymentIssues(queue); err != nil {
		return err
	}

	// Queue will finished if type are Active promotion related
	if queue.IsActivePromotionQueue() || queue.IsPullRequestQueue() {
		return c.updateQueueWithState(queue, s2hv1.Finished)
	}

	// Create queue history
	if err := c.createQueueHistory(queue); err != nil {
		return err
	}

	if err := c.setStableAndSendReport(queue); err != nil {
		return err
	}

	queue.Status.SetCondition(s2hv1.QueueCleaningAfterStarted, corev1.ConditionTrue,
		"starts cleaning the namespace after running task")

	// made queue to clean after state
	return c.updateQueueWithState(queue, s2hv1.CleaningAfter)
}

func (c *controller) setStableAndSendReport(queue *s2hv1.Queue) error {
	isDeploySuccess, isTestSuccess, isReverify := queue.IsDeploySuccess(), queue.IsTestSuccess(), queue.IsReverify()

	compUpgradeStatus := rpc.ComponentUpgrade_UpgradeStatus_FAILURE
	if isDeploySuccess && isTestSuccess && !isReverify {
		// success deploy and test without reverify state
		// save to stable
		if err := c.setStableComponent(queue); err != nil {
			return err
		}

		compUpgradeStatus = rpc.ComponentUpgrade_UpgradeStatus_SUCCESS
	}

	if err := c.sendComponentUpgradeReport(compUpgradeStatus, queue); err != nil {
		return err
	}

	return nil
}

func (c *controller) createQueueHistory(q *s2hv1.Queue) error {
	ctx := context.TODO()

	if err := c.deleteQueueHistoryOutOfRange(ctx, c.namespace); err != nil {
		return err
	}

	now := metav1.Now()
	spec := s2hv1.QueueHistorySpec{
		Queue: &s2hv1.Queue{
			Spec:   q.Spec,
			Status: q.Status,
		},
		StableComponents: c.lastStableComponentList.Items,
		AppliedValues:    c.lastAppliedValues,
		IsDeploySuccess:  q.IsDeploySuccess(),
		IsTestSuccess:    q.IsTestSuccess(),
		IsReverify:       q.IsReverify(),
		CreatedAt:        &now,
	}

	history := &s2hv1.QueueHistory{
		ObjectMeta: metav1.ObjectMeta{
			Name:      q.Status.QueueHistoryName,
			Namespace: c.namespace,
			Labels:    q.Labels,
		},
		Spec: spec,
	}

	fetched := &s2hv1.QueueHistory{}
	err := c.client.Get(ctx, types.NamespacedName{Name: history.Name, Namespace: history.Namespace}, fetched)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			if err := c.client.Create(ctx, history); err != nil {
				logger.Error(err, "cannot create queuehistory")
				return err
			}

			return nil
		}
		logger.Error(err, "cannot get queuehistory")
		return err
	}

	return nil
}

func (c *controller) deleteQueueHistoryOutOfRange(ctx context.Context, namespace string) error {
	queueHists := s2hv1.QueueHistoryList{}
	if err := c.client.List(ctx, &queueHists, &client.ListOptions{Namespace: namespace}); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}

		logger.Error(err, "cannot list queuehistories")
		return errors.Wrapf(err, "cannot list queuehistories in %s", namespace)
	}

	maxHistDays := c.configs.MaxHistoryDays

	// get configuration
	cfg, err := c.getConfiguration()
	if err != nil {
		logger.Error(err, "cannot get configuration")
		return err
	}

	if cfg.Staging != nil && cfg.Staging.MaxHistoryDays != 0 {
		maxHistDays = cfg.Staging.MaxHistoryDays
	}

	// parse max stored queue histories in day to time duration
	maxHistDuration, err := time.ParseDuration(strconv.Itoa(maxHistDays*24) + "h")
	if err != nil {
		logger.Error(err, fmt.Sprintf("cannot parse time duration of %d", maxHistDays))
		return nil
	}

	queueHists.SortDESC()
	now := metav1.Now()
	for i := len(queueHists.Items) - 1; i > 0; i-- {
		if now.Sub(queueHists.Items[i].CreationTimestamp.Time) >= maxHistDuration {
			if err := c.client.Delete(ctx, &queueHists.Items[i]); err != nil {
				if k8serrors.IsNotFound(err) {
					continue
				}

				logger.Error(err, fmt.Sprintf("cannot delete queuehistories %s", queueHists.Items[i].Name))
				return errors.Wrapf(err, "cannot delete queuehistories %s", queueHists.Items[i].Name)
			}
			continue
		}

		break
	}

	return nil
}

// setStableComponent creates or updates StableComponent to match with Queue
func (c *controller) setStableComponent(queue *s2hv1.Queue) (err error) {
	const updatedBy = "samsahai"

	for _, qComp := range queue.Spec.Components {
		stableComp := &s2hv1.StableComponent{}
		err = c.client.Get(
			context.TODO(),
			types.NamespacedName{Namespace: queue.GetNamespace(), Name: qComp.Name},
			stableComp)
		if err != nil && k8serrors.IsNotFound(err) {
			now := metav1.Now()
			stableLabels := internal.GetDefaultLabels(c.teamName)
			stableLabels["app"] = qComp.Name
			stableComp := &s2hv1.StableComponent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      qComp.Name,
					Namespace: queue.Namespace,
					Labels:    stableLabels,
				},
				Spec: s2hv1.StableComponentSpec{
					Name:       qComp.Name,
					Version:    qComp.Version,
					Repository: qComp.Repository,
					UpdatedBy:  updatedBy,
				},
				Status: s2hv1.StableComponentStatus{
					CreatedAt: &now,
					UpdatedAt: &now,
				},
			}
			err = c.client.Create(context.TODO(), stableComp)
			if err != nil {
				logger.Error(err, fmt.Sprintf("cannot create StableComponent: %s/%s", queue.GetNamespace(), qComp.Name))
				return
			}

			continue

		} else if err != nil {
			logger.Error(err, fmt.Sprintf("cannot get StableComponent: %s/%s", queue.GetNamespace(), qComp.Name))
			return err
		}

		if stableComp.Spec.Version == qComp.Version &&
			stableComp.Spec.Repository == qComp.Repository {
			// no change
			continue
		}

		stableComp.Spec.Repository = qComp.Repository
		stableComp.Spec.Version = qComp.Version
		stableComp.Spec.UpdatedBy = updatedBy

		err = c.client.Update(context.TODO(), stableComp)
		if err != nil {
			logger.Error(err, fmt.Sprintf("cannot update StableComponent: %s/%s", queue.GetNamespace(), qComp.Name))
			return
		}
	}

	return nil
}

// createDeploymentZipLogs creates log files in zip format
//
// output is base64 encoded string of the zif file
func (c *controller) createDeploymentZipLogs(q *s2hv1.Queue) (string, error) {
	pods := &corev1.PodList{}
	err := c.client.List(context.TODO(), pods, &client.ListOptions{})
	if err != nil {
		logger.Error(err, "cannot list all pods")
		return "", err
	}

	file, err := os.OpenFile("/tmp/"+q.Status.QueueHistoryName+".zip", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", err
	}

	zipw := zip.NewWriter(file)
	extraArg := ""
	if viper.GetString("kubeconfig") != "" {
		extraArg = " --kubeconfig " + viper.GetString("kubeconfig")
	}
	kubeGetAll := execCommand("kubectl", strings.Split("get po,svc,deploy,sts,rs,job,ing -o wide"+extraArg, " ")...)
	appendFileToZip(zipw, "kube.get.all.txt", kubeGetAll)

	deployEngine := c.getDeployEngine(q)
	yamlValues, _ := deployEngine.GetValues()
	for release, yaml := range yamlValues {
		fileName := fmt.Sprintf("%s-values.yaml", release)
		appendFileToZip(zipw, fileName, yaml)
	}

	for i := range pods.Items {
		pod := pods.Items[i]

		isPodStagingCtrl := strings.Contains(pod.Name, internal.StagingCtrlName)
		if isPodStagingCtrl {
			cmdLogStagingPod := "logs %s --tail=1000 --timestamps%s"
			podStagingCtrlLog := execCommand("kubectl",
				strings.Split(fmt.Sprintf(cmdLogStagingPod, pod.Name, extraArg), " ")...)
			appendFileToZip(zipw, fmt.Sprintf("pod.log.%s.txt", pod.Name), podStagingCtrlLog)
			continue
		}

		isPodRunning := pod.Status.Phase == corev1.PodRunning
		isPodCompleted := pod.Status.Phase == corev1.PodSucceeded
		for _, container := range pod.Status.ContainerStatuses {
			isPodRunning = isPodRunning && container.Ready
		}

		if isPodRunning || isPodCompleted {
			// lets skip running and succeeded pods
			continue
		}

		podDesc := execCommand("kubectl",
			strings.Split(fmt.Sprintf("describe po %s%s", pod.Name, extraArg), " ")...)
		appendFileToZip(zipw,
			fmt.Sprintf("kube.describe.pod.%s.txt", pod.Name),
			podDesc)

		cmdLogPod := "logs %s -c %s --tail=1000 --timestamps%s"
		cmdLogPreviousPod := "logs %s -c %s --tail=1000 --timestamps -p%s"

		for _, container := range pod.Status.InitContainerStatuses {
			if container.RestartCount > 0 || !container.Ready {
				podLog := execCommand("kubectl",
					strings.Split(fmt.Sprintf(cmdLogPod, pod.Name, container.Name, extraArg), " ")...)
				appendFileToZip(zipw, fmt.Sprintf("pod.log.%s.init-container.%s.txt", pod.Name, container.Name), podLog)
				podPrevLog := execCommand("kubectl",
					strings.Split(fmt.Sprintf(cmdLogPreviousPod, pod.Name, container.Name, extraArg), " ")...)
				appendFileToZip(zipw, fmt.Sprintf("pod.pre-log.%s.init-container.%s.txt", pod.Name, container.Name), podPrevLog)
			}
		}

		for _, container := range pod.Status.ContainerStatuses {
			if container.RestartCount > 0 || !container.Ready {
				podLog := execCommand("kubectl",
					strings.Split(fmt.Sprintf(cmdLogPod, pod.Name, container.Name, extraArg), " ")...)
				appendFileToZip(zipw, fmt.Sprintf("pod.log.%s.container.%s.txt", pod.Name, container.Name), podLog)
				podPrevLog := execCommand("kubectl",
					strings.Split(fmt.Sprintf(cmdLogPreviousPod, pod.Name, container.Name, extraArg), " ")...)
				appendFileToZip(zipw, fmt.Sprintf("pod.pre-log.%s.container.%s.txt", pod.Name, container.Name), podPrevLog)
			}
		}
	}

	if err = zipw.Close(); err != nil {
		logger.Warn("error while closing zip: %+v", err)
	}

	if err := file.Close(); err != nil {
		logger.Warn("error while closing file: %+v", err)
	}

	b, err := ioutil.ReadFile("/tmp/" + q.Status.QueueHistoryName + ".zip")
	if err != nil {
		return "", err
	}
	//b := output.Bytes()
	return base64.URLEncoding.EncodeToString(b), nil
}

func appendFileToZip(w *zip.Writer, filename string, data []byte) {
	if data == nil {
		logger.Warnf("no data to zip: %s", filename)
		return
	}
	wr, err := w.Create(filename)
	if err != nil {
		logger.Warn("failed to create entry for %s in zip file: %+v", filename, err)
		return
	}

	if _, err := io.Copy(wr, bytes.NewReader(data)); err != nil {
		logger.Warn("failed to write %s to zip: %+v", filename, err)
	}
}

func execCommand(cmd string, args ...string) []byte {
	out, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		logger.Debug(fmt.Sprintf("`%s %s`: %s (%+v)", cmd, strings.Join(args, " "), string(out), err))
		return nil
	}
	return out
}

func (c *controller) sendComponentUpgradeReport(status rpc.ComponentUpgrade_UpgradeStatus, q *s2hv1.Queue) error {
	var err error
	headers := make(http.Header)
	headers.Set(internal.SamsahaiAuthHeader, c.authToken)
	ctx := context.TODO()
	ctx, err = twirp.WithHTTPRequestHeaders(ctx, headers)
	if err != nil {
		return errors.Wrap(err, "cannot set request header")
	}

	comp := queue.GetComponentUpgradeRPCFromQueue(status, q.Status.QueueHistoryName, c.namespace, q, nil)

	if c.s2hClient != nil {
		_, err = c.s2hClient.RunPostComponentUpgrade(ctx, comp)
		if err != nil {
			logger.Error(err, "cannot send component upgrade report", "queue", q.Spec.Name)
			return errors.Wrap(err, "cannot send component upgrade report")
		}
	}

	return nil
}

func (c *controller) setDeploymentIssues(queue *s2hv1.Queue) error {
	// set deployment issues matching with release label
	deployEngine := c.getDeployEngine(queue)
	parentComps, err := c.configCtrl.GetParentComponents(c.teamName)
	if err != nil {
		return err
	}

	deploymentIssuesMaps := make(map[s2hv1.DeploymentIssueType][]s2hv1.FailureComponent)
	for parentComp := range parentComps {
		ns := c.namespace
		refName := internal.GenReleaseName(ns, parentComp)
		selectors := deployEngine.GetLabelSelectors(refName)
		if len(selectors) == 0 {
			continue
		}

		listOpt := &client.ListOptions{Namespace: ns, LabelSelector: labels.SelectorFromSet(selectors)}

		pods := &corev1.PodList{}
		if err := c.client.List(context.TODO(), pods, listOpt); err != nil {
			logger.Error(err, "cannot list pods")
			return err
		}

		jobs := &batchv1.JobList{}
		if err := c.client.List(context.TODO(), jobs, listOpt); err != nil {
			logger.Error(err, "cannot list jobs")
			return err
		}

		c.extractDeploymentIssues(pods, jobs, deploymentIssuesMaps)
	}

	deploymentIssues := c.convertToDeploymentIssues(deploymentIssuesMaps)
	queue.Status.SetDeploymentIssues(deploymentIssues)

	return nil
}

func (c *controller) extractDeploymentIssues(pods *corev1.PodList, jobs *batchv1.JobList,
	issuesMaps map[s2hv1.DeploymentIssueType][]s2hv1.FailureComponent) {

	for _, pod := range pods.Items {
		compName := c.extractComponentNameFromPod(pod)
		failureComp := s2hv1.FailureComponent{
			ComponentName: compName,
			NodeName:      pod.Spec.NodeName,
		}

		// ignore job pod
		jobFound := false
		for _, podRef := range pod.OwnerReferences {
			if strings.ToLower(podRef.Kind) == "job" {
				jobFound = true
				break
			}
		}

		if !jobFound {
			// check init container issue
			initContainerStatuses := pod.Status.InitContainerStatuses
			initFound := false
			for _, initContainerStatus := range initContainerStatuses {
				if !initContainerStatus.Ready {
					failureComp.FirstFailureContainerName = initContainerStatus.Name
					failureComp.RestartCount = initContainerStatus.RestartCount
					c.appendDeploymentIssues(s2hv1.DeploymentIssueWaitForInitContainer, failureComp, issuesMaps)
					initFound = true
					break
				}
			}
			if initFound {
				continue
			}

			containerStatuses := pod.Status.ContainerStatuses
			found := false
			for _, containerStatus := range containerStatuses {
				if !containerStatus.Ready {
					failureComp.FirstFailureContainerName = containerStatus.Name
					failureComp.RestartCount = containerStatus.RestartCount

					waitingState := containerStatus.State.Waiting
					if waitingState != nil {
						switch waitingState.Reason {
						// check ImagePullBackOff issue
						case "ImagePullBackOff", "ErrImagePull":
							c.appendDeploymentIssues(s2hv1.DeploymentIssueImagePullBackOff, failureComp, issuesMaps)
							found = true

						case "CrashLoopBackOff", "Error":
							c.appendDeploymentIssues(s2hv1.DeploymentIssueCrashLoopBackOff, failureComp, issuesMaps)
							found = true

						case "ContainerCreating":
							c.appendDeploymentIssues(s2hv1.DeploymentIssueContainerCreating, failureComp, issuesMaps)
							found = true
						}
					}

					if found {
						break
					}

					runningState := containerStatus.State.Running
					if runningState != nil {
						found = true

						// if running 0/1, count as CrashLoopBackOff
						if containerStatus.RestartCount > 0 {
							c.appendDeploymentIssues(s2hv1.DeploymentIssueCrashLoopBackOff, failureComp, issuesMaps)
							break
						}

						c.appendDeploymentIssues(s2hv1.DeploymentIssueReadinessProbeFailed, failureComp, issuesMaps)
						found = true
						break
					}

					c.appendDeploymentIssues(s2hv1.DeploymentIssueUndefined, failureComp, issuesMaps)
					found = true
					break
				}
			}

			if found {
				continue
			}

			// check pod pending issue
			if pod.Status.Phase == corev1.PodPending {
				c.appendDeploymentIssues(s2hv1.DeploymentIssuePending, failureComp, issuesMaps)
				continue
			}

			// for other not running pod will be shown as undefined type
			if pod.Status.Phase != corev1.PodRunning {
				c.appendDeploymentIssues(s2hv1.DeploymentIssueUndefined, failureComp, issuesMaps)
				continue
			}
		}
	}

	// check job not complete issue
	for _, job := range jobs.Items {
		failureComp := s2hv1.FailureComponent{
			ComponentName: job.Name,
		}

		if job.Status.CompletionTime == nil {
			failureComp.RestartCount = job.Status.Failed
			c.appendDeploymentIssues(s2hv1.DeploymentIssueJobNotComplete, failureComp, issuesMaps)
		}
	}
}

func (c *controller) extractComponentNameFromPod(pod corev1.Pod) string {
	compName := pod.Name
	for _, podRef := range pod.OwnerReferences {
		if strings.ToLower(podRef.Kind) == "replicaset" {
			rs := &appsv1.ReplicaSet{}
			err := c.client.Get(context.TODO(), types.NamespacedName{Name: podRef.Name, Namespace: pod.Namespace}, rs)
			if err != nil {
				logger.Error(err, "cannot get replicaset %s", podRef.Name)
			}

			for _, rsRef := range rs.OwnerReferences {
				compName = rsRef.Name
			}
			break
		}

		compName = podRef.Name
	}

	if pod.Namespace != "" {
		compName = strings.ReplaceAll(compName, pod.Namespace+"-", "")
	}
	return compName
}

func (c *controller) appendDeploymentIssues(
	issueType s2hv1.DeploymentIssueType,
	failureComp s2hv1.FailureComponent,
	issuesMaps map[s2hv1.DeploymentIssueType][]s2hv1.FailureComponent,
) {

	if _, ok := issuesMaps[issueType]; !ok {
		issuesMaps[issueType] = []s2hv1.FailureComponent{failureComp}
	} else {
		issuesMaps[issueType] = append(issuesMaps[issueType], failureComp)
	}
}

func (c *controller) convertToDeploymentIssues(
	issuesMaps map[s2hv1.DeploymentIssueType][]s2hv1.FailureComponent,
) (issues []s2hv1.DeploymentIssue) {

	issues = make([]s2hv1.DeploymentIssue, 0)
	for issueType, failureComps := range issuesMaps {
		issues = append(issues, s2hv1.DeploymentIssue{
			IssueType:         issueType,
			FailureComponents: failureComps,
		})
	}

	return
}
