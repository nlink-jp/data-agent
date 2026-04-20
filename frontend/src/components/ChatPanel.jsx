import { useState, useEffect, useRef } from "react";
import { SendMessage, ExecuteSQL, GetSession, ApprovePlan, RequestAdditionalAnalysis, FinalizeSession } from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import ResultTable from "./ResultTable";

export default function ChatPanel({ caseId, sessionId }) {
    const [messages, setMessages] = useState([]);
    const [input, setInput] = useState("");
    const [phase, setPhase] = useState("planning");
    const [streaming, setStreaming] = useState("");
    const [sending, setSending] = useState(false);
    const [lastResult, setLastResult] = useState(null);
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
        return () => { unsub1(); unsub2(); unsub3(); };
    }, [sessionId]);

    useEffect(() => {
        if (listRef.current) {
            listRef.current.scrollTop = listRef.current.scrollHeight;
        }
    }, [messages, streaming]);

    const loadSession = async () => {
        try {
            const sess = await GetSession(caseId, sessionId);
            setMessages(sess.chat || []);
            setPhase(sess.phase);
        } catch {}
    };

    const handleSend = async () => {
        if (!input.trim() || sending) return;

        const text = input.trim();
        setInput("");

        // Handle /sql command
        if (text.startsWith("/sql ")) {
            const sql = text.slice(5).trim();
            try {
                const result = await ExecuteSQL(caseId, sessionId, sql);
                setMessages(prev => [
                    ...prev,
                    { role: "user", content: text },
                    { role: "system", content: `Query returned ${result.row_count} rows (${result.duration})` }
                ]);
                setLastResult(result);
            } catch (err) {
                setMessages(prev => [
                    ...prev,
                    { role: "user", content: text },
                    { role: "system", content: `SQL Error: ${err}` }
                ]);
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
        if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            handleSend();
        }
    };

    const handleApprovePlan = async () => {
        try {
            await ApprovePlan(caseId, sessionId);
            loadSession();
        } catch (err) {
            alert(`Error: ${err}`);
        }
    };

    const handleAdditional = async () => {
        try {
            await RequestAdditionalAnalysis(caseId, sessionId);
            loadSession();
        } catch (err) {
            alert(`Error: ${err}`);
        }
    };

    const handleFinalize = async () => {
        try {
            await FinalizeSession(caseId, sessionId);
            loadSession();
        } catch (err) {
            alert(`Error: ${err}`);
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
                {phase === "planning" && (
                    <button onClick={handleApprovePlan} style={{ fontSize: 11 }}>Approve Plan</button>
                )}
                {phase === "review" && (
                    <>
                        <button onClick={handleAdditional} style={{ fontSize: 11 }}>More Analysis</button>
                        <button className="primary" onClick={handleFinalize} style={{ fontSize: 11 }}>Finalize</button>
                    </>
                )}
            </div>

            <div className="message-list" ref={listRef}>
                {messages.map((msg, i) => (
                    <div key={i} className={`message ${msg.role}`}>
                        {msg.content}
                    </div>
                ))}
                {streaming && (
                    <div className="message assistant">{streaming}</div>
                )}
                {lastResult && lastResult.rows && lastResult.rows.length > 0 && (
                    <ResultTable result={lastResult} />
                )}
            </div>

            <div className="chat-input-bar">
                <input
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder={phase === "planning" ? "Describe what you want to analyze..." : "/sql SELECT * FROM ... or ask a question"}
                    disabled={sending || phase === "done"}
                />
                <button className="primary" onClick={handleSend} disabled={sending || phase === "done"}>
                    {sending ? "..." : "Send"}
                </button>
            </div>
        </div>
    );
}
