import { useState, useEffect, useRef, useCallback } from "react";
import { SendMessage, ExecuteSQL, GetSession, ApprovePlan, ReopenSession } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import ResultTable from "./ResultTable";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import InlinePlan, { extractPlanFromContent } from "./InlinePlan";
import StepCard from "./StepCard";

function MessageContent({ content }) {
    if (!content) return null;
    const plan = extractPlanFromContent(content);
    if (!plan) return <Markdown remarkPlugins={[remarkGfm]}>{content}</Markdown>;

    const parts = content.split(/```json\s*\n[\s\S]*?\n```/);
    return (
        <>
            {parts[0]?.trim() && <Markdown remarkPlugins={[remarkGfm]}>{parts[0].trim()}</Markdown>}
            <InlinePlan plan={plan} />
            {parts[1]?.trim() && <Markdown remarkPlugins={[remarkGfm]}>{parts[1].trim()}</Markdown>}
        </>
    );
}

export default function ChatPanel({ caseId, sessionId, onViewReport }) {
    // --- Persistent state (single source: loadSession) ---
    const [session, setSession] = useState(null); // full session object
    const [messages, setMessages] = useState([]);  // derived from session.chat + ephemeral

    // --- Ephemeral UI state ---
    const [streaming, setStreaming] = useState("");
    const [sending, setSending] = useState(false);
    const [ephemeral, setEphemeral] = useState([]); // step progress, report headers during execution
    const [input, setInput] = useState("");
    const listRef = useRef(null);

    // Derived state from session
    const phase = session?.phase || "planning";
    const hasPlan = !!session?.plan;

    // --- Load session: single source of truth ---
    const loadSession = useCallback(async () => {
        if (!caseId || !sessionId) return;
        try {
            const sess = await GetSession(caseId, sessionId);
            setSession(sess);
            setEphemeral([]); // clear ephemeral on reload (persistent data is in session now)
        } catch {}
    }, [caseId, sessionId]);

    useEffect(() => {
        loadSession();
    }, [loadSession]);

    // --- Build display messages from session.chat + ephemeral ---
    useEffect(() => {
        if (!session) { setMessages([]); return; }
        const persistent = (session.chat || []).map(m => {
            if (m.role === "report_header") return { role: "report_header", title: m.content };
            if (m.role === "report_link") {
                const [reportId, ...titleParts] = m.content.split("|");
                return { role: "report_link", reportId, title: titleParts.join("|") };
            }
            return { role: m.role, content: m.content };
        });
        setMessages([...persistent, ...ephemeral]);
    }, [session, ephemeral]);

    // --- Event handlers ---
    useEffect(() => {
        if (!sessionId) return;

        // Stream events: ephemeral, direct UI update
        const unsub1 = EventsOn("chat:stream", (data) => {
            if (data.session === sessionId) {
                setStreaming(prev => prev + data.token);
            }
        });

        const unsub2 = EventsOn("chat:step_progress", (data) => {
            if (data.session === sessionId) {
                setEphemeral(prev => [...prev, { role: "step", stepData: data }]);
            }
        });

        const unsub3 = EventsOn("chat:report_start", (data) => {
            if (data.session === sessionId) {
                setEphemeral(prev => [...prev, { role: "report_header", title: data.title }]);
            }
        });

        // Change notifications: trigger loadSession for authoritative state
        const unsub4 = EventsOn("chat:complete", (data) => {
            if (data.session === sessionId) {
                setStreaming("");
                setSending(false);
                loadSession();
            }
        });

        const unsub5 = EventsOn("session:phase", (data) => {
            if (data.session === sessionId) {
                loadSession();
            }
        });

        const unsub6 = EventsOn("session:plan_detected", (data) => {
            if (data.session === sessionId) {
                loadSession();
            }
        });

        const unsub7 = EventsOn("session:report_ready", (data) => {
            if (data.session === sessionId) {
                loadSession();
            }
        });

        return () => { unsub1(); unsub2(); unsub3(); unsub4(); unsub5(); unsub6(); unsub7(); };
    }, [sessionId, loadSession]);

    // Auto-scroll
    useEffect(() => {
        if (listRef.current) {
            listRef.current.scrollTop = listRef.current.scrollHeight;
        }
    }, [messages, streaming]);

    // --- Actions ---
    const handleSend = async () => {
        if (!input.trim() || sending) return;
        const text = input.trim();
        setInput("");

        // Auto-reopen done sessions
        if (phase === "done") {
            try { await ReopenSession(caseId, sessionId); } catch {}
        }

        if (text.startsWith("/sql ")) {
            const sql = text.slice(5).trim();
            // Optimistic: show user message immediately
            setEphemeral(prev => [...prev, { role: "user", content: text }]);
            try {
                const result = await ExecuteSQL(caseId, sessionId, sql);
                setEphemeral(prev => [...prev, {
                    role: "system",
                    content: `Query returned ${result.row_count} rows`,
                    result: result,
                }]);
                // Reload to persist the exec log
                loadSession();
            } catch (err) {
                setEphemeral(prev => [...prev, { role: "system", content: `SQL Error: ${err}` }]);
            }
            return;
        }

        setSending(true);
        // Optimistic: show user message immediately
        setEphemeral(prev => [...prev, { role: "user", content: text }]);

        try {
            await SendMessage(caseId, sessionId, text);
            // chat:complete event will trigger loadSession
        } catch (err) {
            setSending(false);
            setEphemeral(prev => [...prev, { role: "system", content: `Error: ${err}` }]);
        }
    };

    const handleKeyDown = (e) => {
        if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && !e.isComposing) {
            e.preventDefault();
            handleSend();
        }
    };

    const handleApprovePlan = async () => {
        try {
            await ApprovePlan(caseId, sessionId);
            // session:phase event will trigger loadSession
        } catch (err) {
            setEphemeral(prev => [...prev, { role: "system", content: `Error: ${err}` }]);
        }
    };

    if (!sessionId) {
        return (
            <div className="chat-panel">
                <div className="empty-state">
                    <p>Select or create a session to start analysis</p>
                </div>
            </div>
        );
    }

    return (
        <div className="chat-panel">
            <div className="phase-indicator">
                <span className={`badge ${phase}`}>{phase}</span>
                <span style={{ color: "var(--text-secondary)" }}>Session: {sessionId.slice(0, 8)}</span>
                <div style={{ flex: 1 }} />
            </div>

            <div className="message-list" ref={listRef}>
                {messages.map((msg, i) => {
                    if (msg.role === "step") {
                        return <StepCard key={i} data={msg.stepData} />;
                    }
                    if (msg.role === "report_link") {
                        return (
                            <div key={i} style={{
                                background: "var(--bg-secondary)",
                                border: "1px solid var(--success)",
                                borderRadius: 8, padding: 14, margin: "8px 0", maxWidth: "85%", cursor: "pointer",
                            }} onClick={() => onViewReport && onViewReport(msg.reportId, msg.title)}>
                                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                                    <span style={{ fontSize: 18 }}>📊</span>
                                    <div>
                                        <div style={{ fontSize: 13, fontWeight: 600 }}>Report: {msg.title}</div>
                                        <div style={{ fontSize: 11, color: "var(--accent)", marginTop: 2 }}>Click to view</div>
                                    </div>
                                </div>
                            </div>
                        );
                    }
                    if (msg.role === "report_header") {
                        return (
                            <div key={i} style={{
                                background: "var(--bg-tertiary)", border: "1px solid var(--accent)",
                                borderRadius: 8, padding: "10px 16px", margin: "12px 0 4px 0",
                                display: "flex", alignItems: "center", gap: 8, maxWidth: "85%",
                            }}>
                                <span style={{ fontSize: 18 }}>📊</span>
                                <span style={{ fontSize: 14, fontWeight: 600 }}>{msg.title}</span>
                            </div>
                        );
                    }
                    return (
                        <div key={i}>
                            <div className={`message ${msg.role}`}>
                                <MessageContent content={msg.content} />
                            </div>
                            {msg.result && msg.result.columns && msg.result.rows && msg.result.rows.length > 0 && (
                                <ResultTable result={msg.result} />
                            )}
                        </div>
                    );
                })}
                {streaming && (
                    <div className="message assistant">
                        <Markdown remarkPlugins={[remarkGfm]}>{streaming}</Markdown>
                    </div>
                )}
                {phase === "planning" && hasPlan && !sending && !streaming && (
                    <div style={{
                        background: "var(--bg-secondary)", border: "1px solid var(--success)",
                        borderRadius: 8, padding: 14, margin: "8px 0", maxWidth: "85%",
                    }}>
                        <div style={{ fontSize: 13, marginBottom: 8 }}>
                            Plan is ready. Start execution or continue editing via chat.
                        </div>
                        <button className="primary" onClick={handleApprovePlan} style={{ fontSize: 13, padding: "8px 20px" }}>
                            ▶ Execute Plan
                        </button>
                    </div>
                )}
            </div>

            <div className="chat-input-bar">
                <input
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder={phase === "planning" ? "Describe what you want to analyze... (Cmd+Enter)" : "/sql SELECT * FROM ... (Cmd+Enter)"}
                    disabled={sending}
                />
                <button className="primary" onClick={handleSend} disabled={sending}>
                    {sending ? "..." : "Send"}
                </button>
            </div>
        </div>
    );
}
