import { useState, useEffect, useRef } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";

export default function LogPanel() {
    const [logs, setLogs] = useState([]);
    const panelRef = useRef(null);

    useEffect(() => {
        const unsub = EventsOn("log:entry", (entry) => {
            setLogs(prev => [...prev.slice(-199), entry]);
        });
        return () => unsub();
    }, []);

    useEffect(() => {
        if (panelRef.current) {
            panelRef.current.scrollTop = panelRef.current.scrollHeight;
        }
    }, [logs]);

    return (
        <div className="log-panel" ref={panelRef}>
            {logs.length === 0 ? (
                <div className="log-entry">Waiting for log entries...</div>
            ) : (
                logs.map((entry, i) => (
                    <div key={i} className="log-entry">
                        <span className={`level ${entry.level}`}>{entry.level}</span>
                        {entry.message}
                        {entry.fields && Object.keys(entry.fields).length > 0 && (
                            <span style={{ color: "var(--text-secondary)", marginLeft: 8 }}>
                                {Object.entries(entry.fields).map(([k, v]) => `${k}=${v}`).join(" ")}
                            </span>
                        )}
                    </div>
                ))
            )}
        </div>
    );
}
