const eventStyle = {
    start: { icon: "▶", border: "var(--accent)", bg: "#1f6feb11" },
    done: { icon: "✓", border: "var(--success)", bg: "#3fb95011" },
    failed: { icon: "✗", border: "var(--error)", bg: "#f8514911" },
    skipped: { icon: "−", border: "var(--text-secondary)", bg: "var(--bg-tertiary)" },
};

const typeLabel = {
    sql: "SQL",
    interpret: "Interpret",
    aggregate: "Aggregate",
    container: "Container",
};

export default function StepCard({ data }) {
    if (!data) return null;

    const style = eventStyle[data.event] || eventStyle.start;

    if (data.event === "start") {
        return (
            <div style={{
                padding: "6px 12px",
                borderLeft: `3px solid ${style.border}`,
                background: style.bg,
                borderRadius: "0 6px 6px 0",
                margin: "4px 0",
                fontSize: 12,
                color: "var(--text-secondary)",
            }}>
                {style.icon} <strong>{data.id}</strong> [{typeLabel[data.type] || data.type}] {data.description}
            </div>
        );
    }

    if (data.event === "skipped") {
        return (
            <div style={{
                padding: "6px 12px",
                borderLeft: `3px solid ${style.border}`,
                background: style.bg,
                borderRadius: "0 6px 6px 0",
                margin: "4px 0",
                fontSize: 12,
                color: "var(--text-secondary)",
            }}>
                {style.icon} <strong>{data.id}</strong> skipped
            </div>
        );
    }

    if (data.event === "failed") {
        return (
            <div style={{
                padding: 12,
                borderLeft: `3px solid ${style.border}`,
                background: style.bg,
                borderRadius: "0 6px 6px 0",
                margin: "6px 0",
                maxWidth: "85%",
            }}>
                <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
                    {style.icon} {data.id}: {data.description}
                </div>
                <div style={{ fontSize: 12, color: "var(--error)" }}>
                    Error: {data.error}
                </div>
            </div>
        );
    }

    // done
    return (
        <div style={{
            padding: 12,
            borderLeft: `3px solid ${style.border}`,
            background: style.bg,
            borderRadius: "0 6px 6px 0",
            margin: "6px 0",
            maxWidth: "85%",
        }}>
            <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 6 }}>
                {style.icon} {data.id} [{typeLabel[data.type] || data.type}]: {data.description}
            </div>
            {data.summary && (
                <div style={{
                    fontSize: 12,
                    lineHeight: 1.6,
                    whiteSpace: "pre-wrap",
                    wordBreak: "break-word",
                    color: "var(--text-primary)",
                    background: "var(--bg-primary)",
                    padding: 10,
                    borderRadius: 4,
                    maxHeight: 300,
                    overflow: "auto",
                }}>
                    {data.summary}
                </div>
            )}
        </div>
    );
}
