### Command Usage
#### Notification

##### Global Flags
```
--slack
--slack-token <SLACK_TOKEN>
--slack-channels <SLACK_CHANNELS>
--slack-username <SLACK_USERNAME>
--email
--email-server <SMTP_SERVER>
--email-port <SMTP_PORT>
--email-from <EMAIL_FROM>
--email-to <EMAIL_TO>
--email-subject <EMAIL_SUBJECT>
--rest
--rest-endpoint <REST_ENDPOINT>
```

> **Send component upgrade fail report**
>```
>samsahai send component-upgrade-fail
>--component <REQUIRED_COMPONENT_NAME> 
>--image <REQUIRED_DOCKER_IMAGE>
>--service-owner <REQUIRED_SERVICE_OWNER> 
>--issue-type <ISSUE_TYPE> 
>--values-file-url <VALUES_FILE_URL> 
>--logs-url <LOGS_URL> 
>--error-url <ERROR_URL>
>```  

> **Send active promotion status report**
>```
>samsahai send active-promotion
>--status <REQUIRED_STATUS> 
>--current-active-namespace <REQUIRED_CURRENT_ACTIVE_NAMESPACE>
>--service-owner <REQUIRED_SERVICE_OWNER> 
>--current-active-values-file <CURRENT_ACTIVE_VALUES_FILE> 
>--new-values-file <NEW_VALUES_FILE> 
>--show-detail
>```

> **Send outdated components report**
>```
>samsahai send outdated-components
>--current-active-values-file <REQUIRED_ACTIVE_VALUES_FILE> 
>--new-values-file <REQUIRED_NEW_VALUES_FILE> 
>```

> **Send image missing report**
>```
>samsahai send image-missing --images <REQUIRED_LISTOF_IMAGE_MISSING> 
>```
