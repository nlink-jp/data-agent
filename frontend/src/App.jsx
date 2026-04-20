import { useState, useEffect, useRef, useCallback } from "react";
import "./App.css";
import { ListCases, OpenCase } from "../wailsjs/go/main/App";
import CaseListView from "./components/CaseListView";
import SidePanel from "./components/SidePanel";
import ChatPanel from "./components/ChatPanel";
import LogPanel from "./components/LogPanel";

function App() {
    const [view, setView] = useState("cases");
    const [cases, setCases] = useState([]);
    const [activeCaseId, setActiveCaseId] = useState(null);
    const [activeSessionId, setActiveSessionId] = useState(null);
    const refreshTablesRef = useRef(null);

    const refreshCases = useCallback(async () => {
        try {
            const result = await ListCases();
            setCases(result || []);
        } catch {
            setCases([]);
        }
    }, []);

    useEffect(() => {
        refreshCases();
    }, [refreshCases]);

    const handleOpenCase = async (id) => {
        try {
            await OpenCase(id);
        } catch {} // idempotent — already open is OK
        await refreshCases();
        setActiveCaseId(id);
        setActiveSessionId(null);
        setView("analysis");
    };

    const handleBack = () => {
        setView("cases");
        setActiveCaseId(null);
        setActiveSessionId(null);
        refreshCases();
    };

    const activeCase = cases.find(c => c.id === activeCaseId);

    return (
        <div className="app-layout">
            <div className="app-header">
                <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                    {view === "analysis" && (
                        <button onClick={handleBack} style={{ padding: "4px 8px", fontSize: 12 }}>Back</button>
                    )}
                    <h1>data-agent</h1>
                    {activeCase && (
                        <span style={{ color: "var(--text-secondary)", fontSize: 13 }}>
                            / {activeCase.name}
                        </span>
                    )}
                </div>
                <div className="controls" />
            </div>

            <div className="app-main">
                {view === "cases" ? (
                    <CaseListView
                        cases={cases}
                        onOpenCase={handleOpenCase}
                        refresh={refreshCases}
                    />
                ) : (
                    <div className="analysis-layout">
                        <SidePanel
                            caseId={activeCaseId}
                            activeSessionId={activeSessionId}
                            onSelectSession={setActiveSessionId}
                            onRefreshTables={refreshTablesRef}
                        />
                        <ChatPanel
                            caseId={activeCaseId}
                            sessionId={activeSessionId}
                        />
                    </div>
                )}
            </div>

            <LogPanel />
        </div>
    );
}

export default App;
