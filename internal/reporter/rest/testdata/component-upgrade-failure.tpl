{
    "unixTimestamp": "{{ .UnixTimestamp }}",
    "teamName": "{{ .TeamName }}",
    "issueType": "{{ .IssueTypeStr }}",
    "component": "{{ .Name }}",
    "testBuildTypeID": "{{ .TestRunner.Teamcity.BuildTypeID | ToLower }}",
    "imageRepository": "{{ .Image.Repository }}",
    "imageTag": "{{ .Image.Tag }}",
    "namespace": "{{ .Namespace }}",
    "teamcityURL": "{{ .TestRunner.Teamcity.BuildURL }}",
    "isReverify": "{{ .IsReverify }}"
}
