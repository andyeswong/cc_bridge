package dashboard

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"claude-bridge/internal/storage/repository"
)

type UsageDashboardData struct {
	GeneratedAt string
	Rows        []repository.UsageSummaryRow
}

func RenderUsageDashboard(rows []repository.UsageSummaryRow) (string, error) {
	data := UsageDashboardData{
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		Rows:        rows,
	}

	tpl, err := template.New("dashboard").Funcs(template.FuncMap{
		"formatFloat": func(value float64) string {
			return fmt.Sprintf("%.0f", value)
		},
	}).Parse(usageDashboardTemplate)
	if err != nil {
		return "", err
	}

	var buffer bytes.Buffer

	if err := tpl.Execute(&buffer, data); err != nil {
		return "", err
	}

	return buffer.String(), nil
}

const usageDashboardTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>cc_bridge Dashboard</title>
	<style>
		* {
			box-sizing: border-box;
			margin: 0;
			padding: 0;
		}

		body {
			background: #0f0f1a;
			color: #c8d0e0;
			font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
			padding: 2rem;
			line-height: 1.5;
		}

		.header {
			margin-bottom: 2rem;
		}

		h1 {
			color: #7eb8f7;
			font-size: 1.6rem;
			margin-bottom: .35rem;
			letter-spacing: .02em;
		}

		.meta {
			color: #6b7280;
			font-size: .85rem;
		}

		.cards {
			display: grid;
			grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
			gap: 1rem;
			margin-bottom: 2rem;
		}

		.card {
			background: #151525;
			border: 1px solid #24243a;
			border-radius: 12px;
			padding: 1rem;
		}

		.card-label {
			color: #8899bb;
			font-size: .75rem;
			text-transform: uppercase;
			letter-spacing: .08em;
			margin-bottom: .35rem;
		}

		.card-value {
			color: #ffffff;
			font-size: 1.35rem;
			font-weight: 700;
		}

		h2 {
			color: #8899bb;
			font-size: .95rem;
			margin: 1.5rem 0 .6rem;
			text-transform: uppercase;
			letter-spacing: .08em;
		}

		.table-wrap {
			overflow-x: auto;
			border: 1px solid #24243a;
			border-radius: 12px;
		}

		table {
			width: 100%;
			border-collapse: collapse;
			background: #11111f;
		}

		th {
			background: #151525;
			color: #7eb8f7;
			padding: .75rem 1rem;
			text-align: left;
			border-bottom: 2px solid #252540;
			font-size: .8rem;
			text-transform: uppercase;
			letter-spacing: .06em;
			white-space: nowrap;
		}

		td {
			padding: .75rem 1rem;
			border-bottom: 1px solid #1a1a2e;
			font-size: .9rem;
			white-space: nowrap;
		}

		tr:hover td {
			background: #14142a;
		}

		.num {
			text-align: right;
		}

		.provider-claude {
			color: #80c8ff;
			font-weight: 700;
		}

		.provider-ollama {
			color: #80ffb0;
			font-weight: 700;
		}

		.errors {
			color: #ff9090;
			font-weight: 700;
		}

		.empty {
			color: #6b7280;
			padding: 1rem;
			font-style: italic;
			font-size: .95rem;
		}

		.footer {
			margin-top: 2rem;
			color: #4b5563;
			font-size: .8rem;
		}
	</style>
	<script>
		setTimeout(function () {
			window.location.reload()
		}, 30000)
	</script>
</head>
<body>
	<div class="header">
		<h1>cc_bridge Usage Dashboard</h1>
		<p class="meta">Auto-refreshes every 30 seconds · Last updated: {{ .GeneratedAt }}</p>
	</div>

	{{ if .Rows }}
		<div class="cards">
			<div class="card">
				<div class="card-label">Models</div>
				<div class="card-value">{{ len .Rows }}</div>
			</div>
		</div>

		<h2>Summary by Model</h2>

		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Model</th>
						<th>Provider</th>
						<th class="num">Requests</th>
						<th class="num">Errors</th>
						<th class="num">Prompt Tokens</th>
						<th class="num">Completion Tokens</th>
						<th class="num">Total Tokens</th>
						<th class="num">Avg Duration</th>
					</tr>
				</thead>
				<tbody>
					{{ range .Rows }}
						<tr>
							<td>{{ .Model }}</td>
							<td class="provider-{{ .Provider }}">{{ .Provider }}</td>
							<td class="num">{{ .TotalRequests }}</td>
							<td class="num {{ if gt .Errors 0 }}errors{{ end }}">{{ .Errors }}</td>
							<td class="num">{{ .PromptTokens }}</td>
							<td class="num">{{ .CompletionTokens }}</td>
							<td class="num">{{ .TotalTokens }}</td>
							<td class="num">{{ formatFloat .AvgDurationMs }} ms</td>
						</tr>
					{{ end }}
				</tbody>
			</table>
		</div>
	{{ else }}
		<p class="empty">No requests recorded yet.</p>
	{{ end }}

	<p class="footer">
		Local bridge dashboard · /v1/usage · /v1/usage/recent
	</p>
</body>
</html>`
