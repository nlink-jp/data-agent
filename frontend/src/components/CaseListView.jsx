import { useState } from "react";
import { CreateCase, OpenCase, CloseCase, DeleteCase } from "../../wailsjs/go/main/App";
import { InputDialog, ConfirmDialog } from "./Dialog";

export default function CaseListView({ onOpenCase, refresh, cases }) {
    const [showCreate, setShowCreate] = useState(false);
    const [deleteTarget, setDeleteTarget] = useState(null);

    const handleCreate = async (name) => {
        setShowCreate(false);
        try {
            await CreateCase(name);
            refresh();
        } catch (err) {
            console.error("Create case failed:", err);
        }
    };

    const handleOpen = async (id) => {
        try {
            await OpenCase(id);
            refresh();
            onOpenCase(id);
        } catch (err) {
            console.error("Open case failed:", err);
        }
    };

    const handleClose = async (e, id) => {
        e.stopPropagation();
        try {
            await CloseCase(id);
            refresh();
        } catch (err) {
            console.error("Close case failed:", err);
        }
    };

    const handleDelete = async () => {
        const id = deleteTarget;
        setDeleteTarget(null);
        try {
            await DeleteCase(id);
            refresh();
        } catch (err) {
            console.error("Delete case failed:", err);
        }
    };

    return (
        <div className="case-list">
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
                <h2>Cases</h2>
                <button className="primary" onClick={() => setShowCreate(true)}>+ New Case</button>
            </div>

            {(!cases || cases.length === 0) ? (
                <div className="empty-state" style={{ height: 200 }}>
                    <p>No cases yet. Create one to get started.</p>
                </div>
            ) : (
                <div className="case-grid">
                    {cases.map(c => (
                        <div key={c.id} className="case-card" onClick={() => c.status === "open" ? onOpenCase(c.id) : handleOpen(c.id)}>
                            <div className="name">{c.name}</div>
                            <div className="meta">
                                <span className={`badge ${c.status}`}>{c.status}</span>
                                {" "}Created: {new Date(c.created_at).toLocaleDateString()}
                            </div>
                            <div className="actions">
                                {c.status === "closed" ? (
                                    <button onClick={(e) => { e.stopPropagation(); handleOpen(c.id); }}>Open</button>
                                ) : (
                                    <button onClick={(e) => handleClose(e, c.id)}>Close</button>
                                )}
                                <button onClick={(e) => { e.stopPropagation(); setDeleteTarget(c.id); }}>Delete</button>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {showCreate && (
                <InputDialog
                    title="New Case"
                    placeholder="Case name"
                    onSubmit={handleCreate}
                    onCancel={() => setShowCreate(false)}
                />
            )}

            {deleteTarget && (
                <ConfirmDialog
                    title="Delete Case"
                    message="Delete this case? This cannot be undone."
                    onConfirm={handleDelete}
                    onCancel={() => setDeleteTarget(null)}
                />
            )}
        </div>
    );
}
