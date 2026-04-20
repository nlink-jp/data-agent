const typeLabel = {
    sql: "SQL",
    interpret: "Interpret",
    aggregate: "Aggregate",
    container: "Container",
};

const typeBg = {
    sql: "#1f6feb33",
    interpret: "#d2992233",
    aggregate: "#3fb95033",
    container: "#8b949e33",
};

export default function InlinePlan({ plan, onApprove }) {
    if (!plan) return null;

    return (
        <div style={{
            background: "var(--bg-secondary)",
            border: "1px solid var(--border)",
            borderRadius: 8,
            padding: 14,
            margin: "8px 0",
            maxWidth: "85%",
        }}>
            <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
                Analysis Plan
            </div>
            <div style={{ fontSize: 12, color: "var(--text-secondary)", marginBottom: 12 }}>
                {plan.objective}
            </div>

            {plan.perspectives?.map(p => (
                <div key={p.id} style={{ marginBottom: 10 }}>
                    <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 4 }}>
                        {p.id}: {p.description}
                    </div>
                    {p.steps?.map(s => (
                        <div key={s.id} style={{
                            display: "flex",
                            alignItems: "flex-start",
                            gap: 6,
                            fontSize: 11,
                            padding: "3px 0 3px 10px",
                            borderLeft: "2px solid var(--border)",
                            marginLeft: 4,
                        }}>
                            <span style={{
                                background: typeBg[s.type] || typeBg.container,
                                padding: "1px 5px",
                                borderRadius: 3,
                                fontSize: 10,
                                fontWeight: 600,
                                whiteSpace: "nowrap",
                                flexShrink: 0,
                            }}>
                                {typeLabel[s.type] || s.type}
                            </span>
                            <span>{s.description}</span>
                        </div>
                    ))}
                </div>
            ))}

            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 10 }}>
                <span style={{ fontSize: 11, color: "var(--text-secondary)" }}>
                    {plan.perspectives?.length || 0} perspectives, {plan.perspectives?.reduce((n, p) => n + (p.steps?.length || 0), 0) || 0} steps
                </span>
                {onApprove && (
                    <button className="primary" onClick={onApprove} style={{ fontSize: 12 }}>
                        Approve Plan
                    </button>
                )}
            </div>
        </div>
    );
}

// Try to parse a plan JSON from a message content string
export function extractPlanFromContent(content) {
    if (!content) return null;
    const match = content.match(/```json\s*\n([\s\S]*?)\n```/);
    if (!match) return null;
    try {
        const obj = JSON.parse(match[1]);
        if (obj.objective && obj.perspectives) return obj;
    } catch {}
    return null;
}
