import { useState, useEffect, useRef } from "react";
import { SendMessage, ExecuteSQL, GetSession, ApprovePlan, ReopenSession } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import ResultTable from "./ResultTable";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import InlinePlan, { extractPlanFromContent } from "./InlinePlan";
import StepCard from "./StepCard";

// Render message content, replacing plan JSON with structured InlinePlan
function MessageContent({ content, onApprovePlan, showApprove }) {
    if (!content) return null;

    const plan = extractPlanFromContent(content);
    if (!plan) return <Markdown remarkPlugins={[remarkGfm]}>{content}</Markdown>;

    // Split content around the JSON block
    const parts = content.split(/```json\s*\n[\s\S]*?\n```/);
    return (
        <>
            {parts[0]?.trim() && <Markdown remarkPlugins={[remarkGfm]}>{parts[0].trim()}</Markdown>}
            <InlinePlan plan={plan} onApprove={showApprove ? onApprovePlan : null} />
            {parts[1]?.trim() && <Markdown remarkPlugins={[remarkGfm]}>{parts[1].trim()}</Markdown>}
        </>
    );
}

export default function ChatPanel({ caseId, sessionId, onViewReport }) {
    const [messages, setMessages] = useState([]); // {role, content, result?}
    const [input, setInput] = useState("");
    const [phase, setPhase] = useState("planning");
    const [streaming, setStreaming] = useState("");
    const [sending, setSending] = useState(false);
    const [planDetected, setPlanDetected] = useState(false);
    const listRef = useRef(null);

    useEffect(() => {
        if (!sessionId || !caseId) return;
        loadSession();
    }, [sessionId, caseId]);

    useEffect(() => {
        const unsub1 = EventsOn("chat:stream", (data) => {
            if (data.session === sessionId) {
                setStreaming(prev => prev + data.token);
            }
        });
        const unsub2 = EventsOn("chat:complete", (data) => {
            if (data.session === sessionId) {
                setStreaming("");
                setSending(false);
                loadSession();
            }
        });
        const unsub3 = EventsOn("session:phase", (data) => {
            if (data.session === sessionId) {
                setPhase(data.phase);
            }
        });
        const unsub7 = EventsOn("session:report_ready", (data) => {
            if (data.session === sessionId) {
                setMessages(prev => [...prev, {
                    role: "report_link",
                    reportId: data.report_id,
                    title: data.title,
                }]);
            }
        });
        const unsub6 = EventsOn("chat:report_start", (data) => {
            if (data.session === sessionId) {
                setMessages(prev => [...prev, { role: "report_header", title: data.title }]);
            }
        });
        const unsub5 = EventsOn("chat:step_progress", (data) => {
            if (data.session === sessionId) {
                setMessages(prev => [...prev, { role: "step", stepData: data }]);
            }
        });
        const unsub4 = EventsOn("session:plan_detected", (data) => {
            if (data.session === sessionId) {
                setPlanDetected(true);
                setMessages(prev => [...prev, {
                    role: "system",
                    content: `Plan detected: ${data.objective} (${data.perspectives} perspectives) — Review in side panel, then click "Approve Plan" to proceed.`,
                }]);
            }
        });
        return () => { unsub1(); unsub2(); unsub3(); unsub4(); unsub5(); unsub6(); unsub7(); };
    }, [sessionId]);

    useEffect(() => {
        if (listRef.current) {
            listRef.current.scrollTop = listRef.current.scrollHeight;
        }
    }, [messages, streaming]);

    const loadSession = async () => {
        try {
            const sess = await GetSession(caseId, sessionId);
            setMessages((sess.chat || []).map(m => {
                if (m.role === "report_header") {
                    return { role: "report_header", title: m.content };
                }
                if (m.role === "report_link") {
                    const [reportId, ...titleParts] = m.content.split("|");
                    return { role: "report_link", reportId, title: titleParts.join("|") };
                }
                return { role: m.role, content: m.content };
            }));
            setPhase(sess.phase);
            setPlanDetected(!!sess.plan);
        } catch {}
    };

    const handleSend = async () => {
        if (!input.trim() || sending) return;

        const text = input.trim();
        setInput("");

        // Auto-reopen done sessions
        if (phase === "done") {
            try {
                await ReopenSession(caseId, sessionId);
            } catch {}
        }

        if (text.startsWith("/sql ")) {
            const sql = text.slice(5).trim();
            setMessages(prev => [...prev, { role: "user", content: text }]);
            try {
                const result = await ExecuteSQL(caseId, sessionId, sql);
                setMessages(prev => [...prev, {
                    role: "system",
                    content: `Query returned ${result.row_count} rows`,
                    result: result,
                }]);
            } catch (err) {
                setMessages(prev => [...prev, {
                    role: "system",
                    content: `SQL Error: ${err}`,
                }]);
            }
            return;
        }

        setSending(true);
        setMessages(prev => [...prev, { role: "user", content: text }]);

        try {
            await SendMessage(caseId, sessionId, text);
        } catch (err) {
            setSending(false);
            setMessages(prev => [...prev, { role: "system", content: `Error: ${err}` }]);
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
            loadSession();
        } catch (err) {
            setMessages(prev => [...prev, { role: "system", content: `Error: ${err}` }]);
        }
    };


    const hasApproveInMessages = () => {
        // Check if any message already shows an InlinePlan with Approve button
        for (let i = messages.length - 1; i >= 0; i--) {
            if (messages[i].role === "assistant" && extractPlanFromContent(messages[i].content)) {
                return true;
            }
        }
        return false;
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
                                borderRadius: 8,
                                padding: 14,
                                margin: "8px 0",
                                maxWidth: "85%",
                                cursor: "pointer",
                            }} onClick={() => onViewReport && onViewReport(msg.reportId, msg.title)}>
                                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                                    <span style={{ fontSize: 18 }}>📊</span>
                                    <div>
                                        <div style={{ fontSize: 13, fontWeight: 600 }}>Report Generated: {msg.title}</div>
                                        <div style={{ fontSize: 11, color: "var(--accent)", marginTop: 2 }}>Click to view report</div>
                                    </div>
                                </div>
                            </div>
                        );
                    }
                    if (msg.role === "report_header") {
                        return (
                            <div key={i} style={{
                                background: "var(--bg-tertiary)",
                                border: "1px solid var(--accent)",
                                borderRadius: 8,
                                padding: "10px 16px",
                                margin: "12px 0 4px 0",
                                display: "flex",
                                alignItems: "center",
                                gap: 8,
                                maxWidth: "85%",
                            }}>
                                <span style={{ fontSize: 18 }}>📊</span>
                                <span style={{ fontSize: 14, fontWeight: 600 }}>{msg.title}</span>
                            </div>
                        );
                    }
                    const isLastPlanMsg = planDetected && phase === "planning" &&
                        msg.role === "assistant" && extractPlanFromContent(msg.content) &&
                        i === messages.map((m, idx) => extractPlanFromContent(m.content) ? idx : -1).filter(x => x >= 0).pop();
                    return (
                    <div key={i}>
                        <div className={`message ${msg.role}`}>
                            <MessageContent content={msg.content} onApprovePlan={handleApprovePlan} showApprove={isLastPlanMsg} />
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
                {phase === "planning" && planDetected && !sending && !hasApproveInMessages() && (
                    <div style={{
                        background: "var(--bg-secondary)",
                        border: "1px solid var(--accent)",
                        borderRadius: 8,
                        padding: 14,
                        margin: "8px 0",
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "space-between",
                        maxWidth: "85%",
                    }}>
                        <span style={{ fontSize: 13 }}>Existing plan available. Approve to execute or continue editing.</span>
                        <button className="primary" onClick={handleApprovePlan} style={{ fontSize: 12 }}>Approve Plan</button>
                    </div>
                )}
            </div>

            <div className="chat-input-bar">
                <input
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder={phase === "planning" ? "Describe what you want to analyze... (Cmd+Enter to send)" : "/sql SELECT * FROM ... (Cmd+Enter to send)"}
                    disabled={sending}
                />
                <button className="primary" onClick={handleSend} disabled={sending}>
                    {sending ? "..." : "Send"}
                </button>
            </div>
        </div>
    );
}
