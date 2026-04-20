import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { ExportReport } from "../../wailsjs/go/main/App";
import { InputDialog } from "./Dialog";
import { useState } from "react";

export default function ReportView({ report, caseId, onBack }) {
    const [showExport, setShowExport] = useState(false);

    if (!report) return null;

    const handleExport = async (path) => {
        setShowExport(false);
        try {
            await ExportReport(caseId, report.id, path);
        } catch (err) {
            console.error("Export error:", err);
        }
    };

    return (
        <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
            <div style={{
                padding: "10px 16px",
                background: "var(--bg-secondary)",
                borderBottom: "1px solid var(--border)",
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
            }}>
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <button onClick={onBack} style={{ fontSize: 11, padding: "3px 8px" }}>Back</button>
                    <span style={{ fontSize: 18 }}>📊</span>
                    <span style={{ fontWeight: 600, fontSize: 14 }}>{report.title}</span>
                </div>
                <div style={{ display: "flex", gap: 8 }}>
                    <button onClick={() => navigator.clipboard.writeText(report.content)} style={{ fontSize: 11 }}>
                        Copy Markdown
                    </button>
                    <button onClick={() => setShowExport(true)} style={{ fontSize: 11 }}>
                        Export File
                    </button>
                </div>
            </div>

            <div style={{
                flex: 1,
                overflow: "auto",
                padding: "20px 24px",
            }}>
                <div className="message assistant" style={{
                    maxWidth: "100%",
                    background: "transparent",
                    padding: 0,
                }}>
                    <Markdown remarkPlugins={[remarkGfm]}>{report.content}</Markdown>
                </div>
            </div>

            {showExport && (
                <InputDialog
                    title="Export Report"
                    placeholder="/path/to/report.md"
                    onSubmit={handleExport}
                    onCancel={() => setShowExport(false)}
                />
            )}
        </div>
    );
}
