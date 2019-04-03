package email

const baseTmpl = `{{- define "baseTmpl" -}}
<!DOCTYPE html>
<html lang="en">
<head>
    <link rel="stylesheet" href="https://www.w3schools.com/w3css/4/w3.css">
    <style>
        img {
            display: block;
            margin-left: auto;
            margin-right: auto;
        }
        * {
            margin:5px;
            padding:5px;
        }
    </style>
</head>
<body>
    <div>
        {{- template "content" . }}
    </div>
</body>
</html>
{{- end -}}`

const activePromotionTmpl = `{{- define "content" -}}
    <table class="w3-table-all w3-hoverable">
        <thead>
        <tr colspan="2" class="w3-light-grey">
            <th colspan="2">Active Promotion Status</th>
        </tr>
        </thead>
        <tr>
            <th>Active promotion:</th>
            <td><font color="black">{{ .Status }}</font></td>
        </tr>
        <tr>
            <th>Owner:</th>
            <td><font color="black">{{ .ServiceOwner }}</font></td>
        </tr>
        <tr>
            <th>Current active namespace:</th>
            <td><font color="black">{{ .CurrentActiveNamespace }}</font></td>
        </tr>
        {{- range .Components -}}

        <tr colspan="2">
            <th colspan="2">{{ .CurrentComponent.Name }}</th>
        </tr>
        {{- if ne .OutdatedDays 0 }}
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Not update for</th>
            <td><font color="red">{{ .OutdatedDays }} day(s)</font></td>
        </tr>
        {{- end }}
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Current version:</th>
            <td><font color="black">{{ .CurrentComponent.Version }}</font></td>
        </tr>
        {{- if ne .OutdatedDays 0 }}
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Latest Version:</th>
            <td><font color="black">{{ .NewComponent.Version }}</font></td>
        </tr>
        {{- end }}

        {{- end -}}
    </table>
{{- end -}}`

const componentUpgradeFailTmpl = `{{- define "content" -}}
    <table class="w3-table-all w3-hoverable">
        <thead>
        <tr colspan="2" class="w3-light-grey">
            <th colspan="2">Component Upgrade Fail</th>
        </tr>
        </thead>
        <tr>
            <th>Component Name: </th>
            <td><font color="black">{{ .Component.Name }}</font></td>
        </tr>
        <tr>
            <th>Component Version: </th>
            <td><font color="black">{{ .Component.Image.Tag }}</font></td>
        </tr>
        <tr>
            <th>Component Repository: </th>
            <td><font color="black">{{ .Component.Image.Repository }}</font></td>
        </tr>
        <tr>
            <th>Issue Type</th>
            <td><font color="red">{{ .IssueType }}</font></td>
        </tr>
        <tr>
            <th>Value file Url: </th>
            <td>
            {{- if .ValuesFileURL -}}
                <a href="{{ .ValuesFileURL }}">Click here</a>
            {{- end -}}
            </td>
        </tr>
        <tr>
            <th>CI Url: </th>
            <td>
            {{- if .CIURL -}}
                <a href="{{ .CIURL }}">Click here</a>
            {{- end -}}
            </td>
        </tr>
        <tr>
            <th>Logs: </th>
            <td>
            {{- if .LogsURL -}}
                <a href="{{ .LogsURL }}">Click here</a>
            {{- end -}}
            </td>
        </tr>
        <tr>
            <th>Error </th>
            <td>
            {{- if .ErrorURL -}}
                <a href="{{ .ErrorURL }}">Click here</a>
            {{- end -}}
            </td>
        </tr>
        <tr>
            <th>Owner: </th>
            <td><font color="black">{{ .ServiceOwner }}</font></td>
        </tr>
    </table>
{{- end -}}`

const imageMissingTmpl = `{{- define "content" -}}
    <table class="w3-table-all w3-hoverable">
        <thead>
        <tr colspan="2" class="w3-light-grey">
            <th colspan="2">Image Missing List</th>
        </tr>
        </thead>
        {{- range .Images }}
        <tr>
            <td><font color="black">{{ .Repository }}:{{ .Tag }}</font></td>
        </tr>
        {{- end}}
    </table>
{{- end -}}`

const outdatedComponentsTmpl = `{{- define "content" -}}
    <table class="w3-table-all w3-hoverable">
        <thead>
        <tr colspan="2" class="w3-light-grey">
            <th colspan="2">Component Outdated Summary</th>
        </tr>
        </thead>
        {{- range .Components }}

        {{- if ne .OutdatedDays 0 }}
        <tr colspan="2">
            <th colspan="2">{{ .CurrentComponent.Name }}</th>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Not update for</th>
            <td><font color="red">{{ .OutdatedDays }} day(s)</font></td>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Current version:</th>
            <td><font color="black">{{ .CurrentComponent.Version }}</font></td>
        </tr>
        <tr>
            <th>&emsp;&emsp;&emsp;&emsp;Latest Version:</th>
            <td><font color="black">{{ .NewComponent.Version }}</font></td>
        </tr>
        {{- end }}

        {{- end }}
    </table>
{{- end -}}`
