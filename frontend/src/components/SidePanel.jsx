import { GetTables, ListSessions, CreateSession, ImportData, ListReports, DeleteSession, RenameSession, DeleteReport, RenameReport } from "../../wailsjs/go/main/App";
import { useState, useEffect } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { ImportDialog, InputDialog, ConfirmDialog } from "./Dialog";

export default function SidePanel({ caseId, activeSessionId, onSelectSession, onSelectReport, onRefreshTables }) {
    const [tables, setTables] = useState([]);
    const [sessions, setSessions] = useState([]);
    const [reports, setReports] = useState([]);
    const [showImport, setShowImport] = useState(false);
    const [dialog, setDialog] = useState(null);

    useEffect(() => {
        if (!caseId) return;
        loadData();
    }, [caseId]);

    useEffect(() => {
        if (!activeSessionId || !caseId) return;
        const unsub1 = EventsOn("session:plan_detected", () => loadData());
        const unsub2 = EventsOn("session:phase", () => loadData());
        const unsub3 = EventsOn("chat:complete", () => loadData());
        return () => { unsub1(); unsub2(); unsub3(); };
    }, [activeSessionId, caseId]);

    const loadData = async () => {
        try {
            const t = await GetTables(caseId);
            setTables(t || []);
        } catch { setTables([]); }
        try {
            const s = await ListSessions(caseId);
            setSessions(s || []);
        } catch { setSessions([]); }
        try {
            const r = await ListReports(caseId);
            setReports(r || []);
        } catch { setReports([]); }
    };

    const reportsForSession = (sessionId) =>
        reports.filter(r => r.session_id === sessionId)
            .sort((a, b) => new Date(b.created_at) - new Date(a.created_at));

    const handleImport = async (path, table) => {
        setShowImport(false);
        try {
            await ImportData(caseId, path, table);
            await loadData();
        } catch (err) {
            console.error("Import error:", err);
        }
    };

    const handleNewSession = async () => {
        const sess = await CreateSession(caseId);
        await loadData();
        onSelectSession(sess.id);
    };

    if (onRefreshTables) {
        onRefreshTables.current = loadData;
    }

    return (
        <div className="side-panel">
            <div className="side-section">
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
                    <h3 style={{ margin: 0 }}>Tables</h3>
                    <button onClick={() => setShowImport(true)} style={{ fontSize: 11, padding: "2px 8px" }}>+ Import</button>
                </div>
                {tables.length === 0 ? (
                    <div style={{ color: "var(--text-secondary)", fontSize: 12 }}>No data imported</div>
                ) : (
                    [...tables]
                        .sort((a, b) => a.name.localeCompare(b.name))
                        .map(t => (
                            <div key={t.name} className="side-item">
                                {t.name}
                                <span className="col-type">{t.row_count} rows</span>
                            </div>
                        ))
                )}
            </div>

            <div className="side-section">
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
                    <h3 style={{ margin: 0 }}>Sessions</h3>
                    <button onClick={handleNewSession} style={{ fontSize: 11, padding: "2px 8px" }}>+ New</button>
                </div>
                {sessions.length === 0 ? (
                    <div style={{ color: "var(--text-secondary)", fontSize: 12 }}>No sessions</div>
                ) : (
                    [...sessions]
                        .sort((a, b) => new Date(b.created_at) - new Date(a.created_at))
                        .map(s => {
                            const label = s.plan?.objective
                                ? s.plan.objective
                                : s.chat?.length > 0
                                    ? s.chat.find(m => m.role === "user")?.content || "New session"
                                    : "New session";
                            return (
                                <div
                                    key={s.id}
                                    className="side-item side-item-managed"
                                    style={{
                                        background: s.id === activeSessionId ? "var(--bg-tertiary)" : "transparent",
                                        padding: "6px 8px",
                                    }}
                                    onClick={() => onSelectSession(s.id)}
                                >
                                    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 2 }}>
                                        <span className={`badge ${s.phase}`}>{s.phase}</span>
                                        <span className="item-actions" onClick={e => e.stopPropagation()}>
                                            <button className="icon-btn" title="Rename" onClick={() => setDialog({ type: "rename_session", id: s.id, current: label })}>✎</button>
                                            <button className="icon-btn" title="Delete" onClick={() => setDialog({ type: "delete_session", id: s.id, label })}>×</button>
                                        </span>
                                    </div>
                                    <div style={{ fontSize: 12, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                                        {label.length > 40 ? label.slice(0, 40) + "..." : label}
                                    </div>
                                    <div style={{ fontSize: 10, color: "var(--text-secondary)" }}>
                                        {new Date(s.created_at).toLocaleString()}
                                    </div>
                                    {reportsForSession(s.id).map(r => (
                                        <div
                                            key={r.id}
                                            className="side-item side-item-managed"
                                            onClick={(e) => { e.stopPropagation(); onSelectReport && onSelectReport(r); }}
                                            style={{ padding: "3px 6px", marginTop: 4, marginLeft: 8, borderLeft: "2px solid var(--success)", borderRadius: "0 4px 4px 0" }}
                                        >
                                            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                                                <div style={{ fontSize: 11, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", flex: 1 }}>
                                                    📊 {r.title || "Untitled"}
                                                </div>
                                                <span className="item-actions" onClick={e => e.stopPropagation()}>
                                                    <button className="icon-btn" title="Rename" onClick={() => setDialog({ type: "rename_report", id: r.id, current: r.title })}>✎</button>
                                                    <button className="icon-btn" title="Delete" onClick={() => setDialog({ type: "delete_report", id: r.id, label: r.title })}>×</button>
                                                </span>
                                            </div>
                                            <div style={{ fontSize: 9, color: "var(--text-secondary)" }}>
                                                {new Date(r.created_at).toLocaleString()}
                                            </div>
                                        </div>
                                    ))}
                                </div>
                            );
                        })
                )}
            </div>

            {showImport && (
                <ImportDialog
                    onSubmit={handleImport}
                    onCancel={() => setShowImport(false)}
                />
            )}

            {dialog?.type === "rename_session" && (
                <InputDialog
                    title="Rename Session"
                    placeholder={dialog.current}
                    onSubmit={async (name) => {
                        await RenameSession(caseId, dialog.id, name);
                        setDialog(null);
                        loadData();
                    }}
                    onCancel={() => setDialog(null)}
                />
            )}

            {dialog?.type === "delete_session" && (
                <ConfirmDialog
                    title="Delete Session"
                    message={`Delete "${dialog.label}"?`}
                    onConfirm={async () => {
                        await DeleteSession(caseId, dialog.id);
                        setDialog(null);
                        if (dialog.id === activeSessionId) onSelectSession(null);
                        loadData();
                    }}
                    onCancel={() => setDialog(null)}
                />
            )}

            {dialog?.type === "rename_report" && (
                <InputDialog
                    title="Rename Report"
                    placeholder={dialog.current}
                    onSubmit={async (name) => {
                        await RenameReport(caseId, dialog.id, name);
                        setDialog(null);
                        loadData();
                    }}
                    onCancel={() => setDialog(null)}
                />
            )}

            {dialog?.type === "delete_report" && (
                <ConfirmDialog
                    title="Delete Report"
                    message={`Delete "${dialog.label}"?`}
                    onConfirm={async () => {
                        await DeleteReport(caseId, dialog.id);
                        setDialog(null);
                        loadData();
                    }}
                    onCancel={() => setDialog(null)}
                />
            )}
        </div>
    );
}
