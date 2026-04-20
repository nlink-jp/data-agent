import { useState, useEffect } from "react";
import { GetConfig, SaveConfig } from "../../wailsjs/go/main/App";

export default function SettingsView() {
    const [cfg, setCfg] = useState(null);
    const [saving, setSaving] = useState(false);
    const [message, setMessage] = useState("");

    useEffect(() => {
        GetConfig().then(c => setCfg(c));
    }, []);

    const handleSave = async () => {
        setSaving(true);
        setMessage("");
        try {
            await SaveConfig(cfg);
            setMessage("Settings saved.");
        } catch (err) {
            setMessage(`Error: ${err}`);
        }
        setSaving(false);
    };

    if (!cfg) return <div style={{ padding: 24 }}>Loading...</div>;

    const update = (section, key, value) => {
        if (key === null) {
            // Top-level field (e.g., theme)
            setCfg(prev => ({ ...prev, [section]: value }));
        } else {
            setCfg(prev => ({
                ...prev,
                [section]: { ...prev[section], [key]: value },
            }));
        }
    };

    const updateNum = (section, key, value) => {
        const n = parseFloat(value);
        if (!isNaN(n)) update(section, key, n);
    };

    return (
        <div style={{ flex: 1, overflow: "auto", padding: 24, maxWidth: 600 }}>
            <h2 style={{ fontSize: 18, marginBottom: 20 }}>Settings</h2>

            <Section title="Appearance">
                <Field label="Theme">
                    <select
                        value={cfg.theme || "dark"}
                        onChange={e => {
                            update("theme", null, e.target.value);
                            document.documentElement.setAttribute("data-theme", e.target.value);
                        }}
                        style={{ background: "var(--bg-secondary)", color: "var(--text-primary)", border: "1px solid var(--border)", padding: "6px 10px", borderRadius: 6, width: "100%" }}
                    >
                        <option value="dark">Dark</option>
                        <option value="light">Light</option>
                        <option value="warm">Warm</option>
                        <option value="midnight">Midnight</option>
                    </select>
                </Field>
            </Section>

            <Section title="LLM Backend">
                <Field label="Backend">
                    <select
                        value={cfg.llm?.backend || "local"}
                        onChange={e => update("llm", "backend", e.target.value)}
                        style={{ background: "var(--bg-secondary)", color: "var(--text-primary)", border: "1px solid var(--border)", padding: "6px 10px", borderRadius: 6, width: "100%" }}
                    >
                        <option value="local">Local LLM (OpenAI-compatible)</option>
                        <option value="vertex_ai">Vertex AI (Gemini)</option>
                    </select>
                </Field>
            </Section>

            <Section title="Vertex AI">
                <Field label="Project ID">
                    <input value={cfg.vertex_ai?.project || ""} onChange={e => update("vertex_ai", "project", e.target.value)} />
                </Field>
                <Field label="Region">
                    <input value={cfg.vertex_ai?.region || ""} onChange={e => update("vertex_ai", "region", e.target.value)} />
                </Field>
                <Field label="Model">
                    <input value={cfg.vertex_ai?.model || ""} onChange={e => update("vertex_ai", "model", e.target.value)} />
                </Field>
            </Section>

            <Section title="Local LLM">
                <Field label="Endpoint">
                    <input value={cfg.local_llm?.endpoint || ""} onChange={e => update("local_llm", "endpoint", e.target.value)} />
                </Field>
                <Field label="Model">
                    <input value={cfg.local_llm?.model || ""} onChange={e => update("local_llm", "model", e.target.value)} />
                </Field>
                <Field label="API Key">
                    <input type="password" value={cfg.local_llm?.api_key || ""} onChange={e => update("local_llm", "api_key", e.target.value)} />
                </Field>
            </Section>

            <Section title="Analysis">
                <Field label="Context Limit (tokens)">
                    <input type="number" value={cfg.analysis?.context_limit || 131072} onChange={e => updateNum("analysis", "context_limit", e.target.value)} />
                </Field>
                <Field label="Max Records Per Window">
                    <input type="number" value={cfg.analysis?.max_records_per_window || 200} onChange={e => updateNum("analysis", "max_records_per_window", e.target.value)} />
                </Field>
                <Field label="Overlap Ratio">
                    <input type="number" step="0.05" value={cfg.analysis?.overlap_ratio || 0.1} onChange={e => updateNum("analysis", "overlap_ratio", e.target.value)} />
                </Field>
                <Field label="Max Findings">
                    <input type="number" value={cfg.analysis?.max_findings || 100} onChange={e => updateNum("analysis", "max_findings", e.target.value)} />
                </Field>
            </Section>

            <Section title="Container">
                <Field label="Runtime">
                    <select
                        value={cfg.container?.runtime || "podman"}
                        onChange={e => update("container", "runtime", e.target.value)}
                        style={{ background: "var(--bg-secondary)", color: "var(--text-primary)", border: "1px solid var(--border)", padding: "6px 10px", borderRadius: 6, width: "100%" }}
                    >
                        <option value="podman">Podman</option>
                        <option value="docker">Docker</option>
                    </select>
                </Field>
                <Field label="Image">
                    <input value={cfg.container?.image || ""} onChange={e => update("container", "image", e.target.value)} />
                </Field>
            </Section>

            <div style={{ marginTop: 20, display: "flex", alignItems: "center", gap: 12 }}>
                <button className="primary" onClick={handleSave} disabled={saving}>
                    {saving ? "Saving..." : "Save Settings"}
                </button>
                {message && <span style={{ fontSize: 12, color: message.startsWith("Error") ? "var(--error)" : "var(--success)" }}>{message}</span>}
            </div>
        </div>
    );
}

function Section({ title, children }) {
    return (
        <div style={{ marginBottom: 20 }}>
            <h3 style={{ fontSize: 13, color: "var(--text-secondary)", textTransform: "uppercase", marginBottom: 8, borderBottom: "1px solid var(--border)", paddingBottom: 4 }}>{title}</h3>
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>{children}</div>
        </div>
    );
}

function Field({ label, children }) {
    return (
        <div>
            <label style={{ fontSize: 12, color: "var(--text-secondary)", display: "block", marginBottom: 2 }}>{label}</label>
            {children}
        </div>
    );
}
