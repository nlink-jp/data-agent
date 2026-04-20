import { GetTables, ListSessions, CreateSession, ImportData } from "../../wailsjs/go/main/App";
import { useState, useEffect } from "react";
import { ImportDialog } from "./Dialog";

export default function SidePanel({ caseId, activeSessionId, onSelectSession, onRefreshTables }) {
    const [tables, setTables] = useState([]);
    const [sessions, setSessions] = useState([]);
    const [showImport, setShowImport] = useState(false);

    useEffect(() => {
        if (!caseId) return;
        loadData();
    }, [caseId]);

    const loadData = async () => {
        try {
            const t = await GetTables(caseId);
            setTables(t || []);
        } catch { setTables([]); }
        try {
            const s = await ListSessions(caseId);
            setSessions(s || []);
        } catch { setSessions([]); }
    };

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
                    tables.map(t => (
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
                    sessions.map(s => (
                        <div
                            key={s.id}
                            className="side-item"
                            style={{ background: s.id === activeSessionId ? "var(--bg-tertiary)" : "transparent" }}
                            onClick={() => onSelectSession(s.id)}
                        >
                            <span className={`badge ${s.phase}`}>{s.phase}</span>
                            {" "}{s.id.slice(0, 8)}
                        </div>
                    ))
                )}
            </div>

            {showImport && (
                <ImportDialog
                    onSubmit={handleImport}
                    onCancel={() => setShowImport(false)}
                />
            )}
        </div>
    );
}
