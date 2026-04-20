import { ListCases, CreateCase, OpenCase, CloseCase, DeleteCase } from "../../wailsjs/go/main/App";

export default function CaseListView({ onOpenCase, refresh, cases }) {
    const handleCreate = async () => {
        const name = prompt("Case name:");
        if (!name) return;
        await CreateCase(name);
        refresh();
    };

    const handleOpen = async (id) => {
        await OpenCase(id);
        refresh();
        onOpenCase(id);
    };

    const handleClose = async (e, id) => {
        e.stopPropagation();
        await CloseCase(id);
        refresh();
    };

    const handleDelete = async (e, id) => {
        e.stopPropagation();
        if (!confirm("Delete this case? This cannot be undone.")) return;
        await DeleteCase(id);
        refresh();
    };

    return (
        <div className="case-list">
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
                <h2>Cases</h2>
                <button className="primary" onClick={handleCreate}>+ New Case</button>
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
                                <button onClick={(e) => handleDelete(e, c.id)}>Delete</button>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}
