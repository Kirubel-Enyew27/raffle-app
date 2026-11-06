from pathlib import Path

b = Path('/home/kiru/projects/raffle-app/frontend/src/features/admin')

def fix_export_block(s: str) -> str:
    lines = s.splitlines(keepends=True)
    out = []
    in_func = False
    for line in lines:
        if 'function doExport()' in line:
            in_func = True
        if in_func and line.strip().startswith('('):
            out.append('    const lines = [arguments[0], ...arguments[1].map(r => r.map(v => `" + v + "`).join(","))]\n')
            out.append("    const blob = new Blob([lines], { type: 'text/csv' })\n")
            out.append('    const a = document.createElement("a")\n')
            out.append('    a.href = URL.createObjectURL(blob)\n')
            out.append("    a.download = 'profit-summary.csv'\n")
            out.append('    a.click()\n')
            out.append('    URL.revokeObjectURL(a.href)\n')
            in_func = False
        else:
            out.append(line)
    return ''.join(out)

for name in ['ProfitSummaryReport.tsx', 'RevenueReport.tsx', 'TicketsReport.tsx', 'WinnerSummaryReport.tsx']:
    p = b / name
    s = p.read_text()
    s = s.replace("import { exportCSV } from './ReportsControls' //", '')
    s = s.replace('exportCSV', '')
    s = fix_export_block(s)
    p.write_text(s)
print('done')
