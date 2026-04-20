import { useState, useEffect, useRef, useCallback } from "react";
import "./App.css";
import { ListCases, OpenCase, ListReports } from "../wailsjs/go/main/App";
import CaseListView from "./components/CaseListView";
import SidePanel from "./components/SidePanel";
import ChatPanel from "./components/ChatPanel";
import ReportView from "./components/ReportView";
import LogPanel from "./components/LogPanel";
import SettingsView from "./components/SettingsView";

function App() {
    const [view, setView] = useState("cases"); // "cases", "analysis", "report", "settings"
    const [cases, setCases] = useState([]);
    const [activeCaseId, setActiveCaseId] = useState(null);
    const [activeSessionId, setActiveSessionId] = useState(null);
    const [activeReport, setActiveReport] = useState(null);
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
        } catch {}
        await refreshCases();
        setActiveCaseId(id);
        setActiveSessionId(null);
        setActiveReport(null);
        setView("analysis");
    };

    const handleBack = () => {
        setView("cases");
        setActiveCaseId(null);
        setActiveSessionId(null);
        setActiveReport(null);
        refreshCases();
    };

    const handleSelectReport = (report) => {
        setActiveReport(report);
        setView("report");
    };

    const handleBackFromReport = () => {
        setActiveReport(null);
        setView("analysis");
    };

    const handleViewReportById = async (reportId, title) => {
        try {
            const reports = await ListReports(activeCaseId);
            const found = (reports || []).find(r => r.id === reportId);
            if (found) {
                handleSelectReport(found);
            }
        } catch {}
    };

    const activeCase = cases.find(c => c.id === activeCaseId);

    const renderMainContent = () => {
        if (view === "report" && activeReport) {
            return <ReportView report={activeReport} caseId={activeCaseId} onBack={handleBackFromReport} />;
        }
        return (
            <ChatPanel
                caseId={activeCaseId}
                sessionId={activeSessionId}
                onViewReport={handleViewReportById}
            />
        );
    };

    return (
        <div className="app-layout">
            <div className="app-header">
                <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                    {(view === "analysis" || view === "report") && (
                        <button onClick={handleBack} style={{ padding: "4px 8px", fontSize: 12 }}>Back</button>
                    )}
                    <h1>data-agent</h1>
                    {activeCase && (
                        <span style={{ color: "var(--text-secondary)", fontSize: 13 }}>
                            / {activeCase.name}
                        </span>
                    )}
                </div>
                <div className="controls">
                    <button onClick={() => setView(view === "settings" ? "cases" : "settings")} style={{ fontSize: 12 }}>
                        {view === "settings" ? "Close Settings" : "Settings"}
                    </button>
                </div>
            </div>

            <div className="app-main">
                {view === "settings" ? (
                    <SettingsView onClose={() => setView("cases")} />
                ) : view === "cases" ? (
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
                            onSelectSession={(id) => { setActiveSessionId(id); setActiveReport(null); setView("analysis"); }}
                            onSelectReport={handleSelectReport}
                            onRefreshTables={refreshTablesRef}
                        />
                        {renderMainContent()}
                    </div>
                )}
            </div>

            <LogPanel />
        </div>
    );
}

export default App;
