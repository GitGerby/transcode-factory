package main
const html_template = `
<!DOCTYPE html>
<html>
<head>
<meta http-equiv="refresh" content="10">
<style>
	table, td, th {
		border: 1px solid;
	}

	table {
		width: 100%;
		border-collapse: collapse;
	}
}

</style>
</head>
<body>
	<h1>transcode-factory status</h1><br>
	<h2>Active Jobs</h2>
	Currently running jobs: {{len .ActiveJobs}}
	<table>
		{{range .ActiveJobs}}
		<tr>
			<th style="text-align:left">
				Job ID:
			</th>
			<td>
				{{.Id}}
			</td>
			<th style="text-align:left">
				Stage:
			</th>
			<td>
				{{.State}}
			</td>
		</tr>
		<tr>
			<th style="text-align:left">
				Source:
			</th>
			<td> 
				{{.JobDefinition.Source}} 
			</td>
			<th style="text-align:left">
				Codec:
			</th>
			<td>
			{{.SourceMeta.Codec}} 
			</td>
		</tr>
		<tr>
			<th style="text-align:left">
				Destination:
			</th>
			<td>
				{{.JobDefinition.Destination}}<br>
			</td>
			<th style="text-align:left">
				Codec / Crf:
			</th>
			<td>
				{{.JobDefinition.Codec}} / {{.JobDefinition.Crf}}
			</td>
		</tr>
		<tr>
			<th style="text-align:left">
				Subtitles:
			</th>
			<td colspan = "0">
				<ol>
					{{range .JobDefinition.Srt_files}}
						<li>{{.}}</li>
					{{end}}
				</ol>
			</td>
			<th style="text-align:left">
				Video Filter:
			</th>
			<td>
				{{.JobDefinition.Video_filters}}
			</td>
		</tr>
		{{end}}
	</table>
	<h2>Current Queue</h2><br>
	Queue Length: {{len .TranscodeQueue}}
  <table>
    <tr>
      <th>Job ID</th>
      <th>Source</th>
      <th>Destination</th>
      <th>CRF</th>
      <th>autocrop</th>
      <th>SRT Files</th>
    </tr>
    {{range .TranscodeQueue}}
      <tr>
        <td>{{.Id}}</td>
        <td>{{.JobDefinition.Source}}</td>
        <td>{{.JobDefinition.Destination}}</td>
        <td>{{.JobDefinition.Crf}}</td>
        <td>{{.JobDefinition.Autocrop}}</td>
        <td>
        {{range .JobDefinition.Srt_files}}
        {{.}}<br>
        {{end}}</td>
      </tr>
    {{end}}
  </table>
</body>
`
