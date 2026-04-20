export default function ResultTable({ result }) {
    if (!result || !result.columns || !result.rows) return null;

    const formatValue = (v) => {
        if (v === null || v === undefined) return "NULL";
        if (typeof v === "object") return JSON.stringify(v);
        return String(v);
    };

    return (
        <div className="result-table-container">
            <table className="result-table">
                <thead>
                    <tr>
                        {result.columns.map(col => (
                            <th key={col}>{col}</th>
                        ))}
                    </tr>
                </thead>
                <tbody>
                    {result.rows.map((row, i) => (
                        <tr key={i}>
                            {result.columns.map(col => (
                                <td key={col}>{formatValue(row[col])}</td>
                            ))}
                        </tr>
                    ))}
                </tbody>
            </table>
            <div style={{ color: "var(--text-secondary)", fontSize: 11, marginTop: 4 }}>
                {result.row_count} row(s)
            </div>
        </div>
    );
}
