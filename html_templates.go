package main

const html_template = `
<!DOCTYPE html>
<html>
<head>
<meta http-equiv="refresh" content="600">
<style>
  table {width: 100%; border-collapse: collapse;}
  td, th {border: 1px solid;}
	.blank_row tr, .blank_row td {border:none;}
  .highlight_job:hover {background-color: coral;}
</style>
</head>
<body>
	<h2>Active Jobs</h2>
	Currently running jobs: {{len .ActiveJobs}}
	<table>
		{{range .ActiveJobs}}
		<tbody class="highlight_job">
				<tr>
					<th style="text-align:left">Job ID:</th>
					<td>{{.Id}}</td>
					<th style="text-align:left">Stage:</th>
					<td>{{.State}}</td>
				</tr>
				<tr>
					<th style="text-align:left">Source:</th>
					<td>{{.JobDefinition.Source}}</td>
					<th style="text-align:left">Codec:</th>
					<td>{{.SourceMeta.Codec}}</td>
				</tr>
				<tr>
					<th style="text-align:left">Destination:</th>
					<td>{{.JobDefinition.Destination}}</td>
					<th style="text-align:left">Codec / Crf:</th>
					<td>{{.JobDefinition.Codec}} / {{.JobDefinition.Crf}}</td>
				</tr>
				<tr>
					<th style="text-align:left">Subtitles:</th>
					<td colspan = "0">
						<ol>
							{{range .JobDefinition.Srt_files}}
								<li>{{.}}</li>
							{{end}}
						</ol>
					</td>
					<th style="text-align:left">Video Filter:</th>
					<td>{{.JobDefinition.Video_filters}}</td>
				</tr>
				<tr>
					<th style="text-align:left">Log Output:</th>
					<td id="log-{{.Id}}"></td>
					<th style="text-align:left">Duration:</th>
					<td>{{.SourceMeta.Duration}}</td>
				</tr>
		</tbody>
		<tbody>
        <tr class="blank_row"><td colspan="4" style="height: 1em;"></td></tr>
		</tbody>
		{{end}}
	</table>
	<h2>Current Queue</h2><br>
	Queue Length: {{len .QueuedJobs}}
  <table>
    <tr>
      <th>Job ID</th>
      <th>Source</th>
      <th>Destination</th>
      <th>CRF</th>
      <th>autocrop</th>
      <th>SRT Files</th>
    </tr>
    {{range .QueuedJobs}}
      <tr>
        <td>{{.Id}}</td>
        <td>{{.JobDefinition.Source}}</td>
        <td>{{.JobDefinition.Destination}}</td>
        <td>{{.JobDefinition.Crf}}</td>
        <td>{{.CropState}}</td>
        <td>
        {{range .JobDefinition.Srt_files}}
        {{.}}<br>
        {{end}}</td>
      </tr>
    {{end}}
  </table>
	    <script>
				var loc = window.location.hostname;
				var ws_uri = "ws://" + loc + ":51218/logstream";
        var ws = new WebSocket(ws_uri);

        ws.onmessage = function(event) {
					var statusMessage = JSON.parse(event.data);

						if (statusMessage.RefreshNeeded == true) {
							ws.close();
							location.reload();
						}

						{{range .ActiveJobs}}
						if (statusMessage.LogMessages[{{.Id}}]) {
							document.getElementById("log-{{.Id}}").innerText = statusMessage.LogMessages[{{.Id}}];
						}
						{{end}}

        };

        ws.onerror = function(err) {
            console.log("WebSocket Error:", err);
        };
    </script>
</body>
`
