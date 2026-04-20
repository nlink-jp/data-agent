import { useState, useEffect, useRef } from "react";

export function InputDialog({ title, placeholder, onSubmit, onCancel }) {
    const [value, setValue] = useState("");
    const inputRef = useRef(null);

    useEffect(() => {
        inputRef.current?.focus();
    }, []);

    const handleSubmit = () => {
        if (value.trim()) {
            onSubmit(value.trim());
        }
    };

    const handleKeyDown = (e) => {
        if (e.key === "Enter") handleSubmit();
        if (e.key === "Escape") onCancel();
    };

    return (
        <div className="dialog-overlay" onClick={onCancel}>
            <div className="dialog" onClick={e => e.stopPropagation()}>
                <h3>{title}</h3>
                <input
                    ref={inputRef}
                    value={value}
                    onChange={e => setValue(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder={placeholder}
                    style={{ marginTop: 12 }}
                />
                <div className="dialog-actions">
                    <button onClick={onCancel}>Cancel</button>
                    <button className="primary" onClick={handleSubmit}>OK</button>
                </div>
            </div>
        </div>
    );
}

export function ConfirmDialog({ title, message, onConfirm, onCancel }) {
    useEffect(() => {
        const handler = (e) => {
            if (e.key === "Enter") onConfirm();
            if (e.key === "Escape") onCancel();
        };
        window.addEventListener("keydown", handler);
        return () => window.removeEventListener("keydown", handler);
    }, [onConfirm, onCancel]);

    return (
        <div className="dialog-overlay" onClick={onCancel}>
            <div className="dialog" onClick={e => e.stopPropagation()}>
                <h3>{title}</h3>
                <p style={{ marginTop: 8, color: "var(--text-secondary)" }}>{message}</p>
                <div className="dialog-actions">
                    <button onClick={onCancel}>Cancel</button>
                    <button className="primary" onClick={onConfirm}>OK</button>
                </div>
            </div>
        </div>
    );
}

export function ImportDialog({ onSubmit, onCancel }) {
    const [path, setPath] = useState("");
    const [table, setTable] = useState("");
    const pathRef = useRef(null);

    useEffect(() => {
        pathRef.current?.focus();
    }, []);

    const handleSubmit = () => {
        if (path.trim() && table.trim()) {
            onSubmit(path.trim(), table.trim());
        }
    };

    const handleKeyDown = (e) => {
        if (e.key === "Enter") handleSubmit();
        if (e.key === "Escape") onCancel();
    };

    return (
        <div className="dialog-overlay" onClick={onCancel}>
            <div className="dialog" onClick={e => e.stopPropagation()}>
                <h3>Import Data</h3>
                <div style={{ marginTop: 12, display: "flex", flexDirection: "column", gap: 8 }}>
                    <label style={{ fontSize: 12, color: "var(--text-secondary)" }}>File path (CSV, JSON, JSONL)</label>
                    <input
                        ref={pathRef}
                        value={path}
                        onChange={e => setPath(e.target.value)}
                        onKeyDown={handleKeyDown}
                        placeholder="/path/to/data.csv"
                    />
                    <label style={{ fontSize: 12, color: "var(--text-secondary)" }}>Table name</label>
                    <input
                        value={table}
                        onChange={e => setTable(e.target.value)}
                        onKeyDown={handleKeyDown}
                        placeholder="my_table"
                    />
                </div>
                <div className="dialog-actions">
                    <button onClick={onCancel}>Cancel</button>
                    <button className="primary" onClick={handleSubmit}>Import</button>
                </div>
            </div>
        </div>
    );
}
