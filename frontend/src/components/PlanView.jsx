const statusIcon = {
    planned: "○",
    running: "▶",
    done: "✓",
    failed: "✗",
    skipped: "−",
    revised: "↻",
};

const typeLabel = {
    sql: "SQL",
    interpret: "解釈",
    aggregate: "統合",
    container: "実行",
};

export default function PlanView({ plan }) {
    if (!plan) return null;

    const totalSteps = plan.perspectives?.reduce((n, p) => n + (p.steps?.length || 0), 0) || 0;

    return (
        <div className="side-section">
            <h3>Plan</h3>
            <div style={{ fontSize: 12, color: "var(--text-secondary)", marginBottom: 8 }}>
                {plan.objective}
            </div>
            <div style={{ fontSize: 11, color: "var(--text-secondary)", marginBottom: 8 }}>
                {plan.perspectives?.length || 0} perspectives / {totalSteps} steps / v{plan.version || 1}
            </div>

            {plan.perspectives?.map(p => (
                <div key={p.id} style={{ marginBottom: 8 }}>
                    <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 4 }}>
                        {p.id}: {p.description}
                    </div>
                    {p.steps?.map(s => (
                        <div key={s.id} style={{
                            fontSize: 11,
                            padding: "2px 0 2px 12px",
                            color: s.status === "done" ? "var(--success)" :
                                   s.status === "failed" ? "var(--error)" :
                                   s.status === "skipped" ? "var(--text-secondary)" :
                                   "var(--text-primary)",
                        }}>
                            <span style={{ marginRight: 4 }}>{statusIcon[s.status] || "○"}</span>
                            <span style={{
                                background: "var(--bg-tertiary)",
                                padding: "0 4px",
                                borderRadius: 3,
                                marginRight: 4,
                                fontSize: 10,
                            }}>
                                {typeLabel[s.type] || s.type}
                            </span>
                            {s.description}
                        </div>
                    ))}
                </div>
            ))}
        </div>
    );
}
