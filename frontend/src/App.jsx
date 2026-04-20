import {useState, useEffect} from 'react';
import './App.css';
import {ListCases, CreateCase} from "../wailsjs/go/main/App";
import {EventsOn} from "../wailsjs/runtime/runtime";

function App() {
    const [cases, setCases] = useState([]);
    const [logs, setLogs] = useState([]);

    useEffect(() => {
        ListCases().then(result => {
            if (result) setCases(result);
        });
        EventsOn("log:entry", (entry) => {
            setLogs(prev => [...prev.slice(-49), entry]);
        });
    }, []);

    const handleCreate = () => {
        const name = prompt("Case name:");
        if (name) {
            CreateCase(name).then(() => {
                ListCases().then(result => {
                    if (result) setCases(result);
                });
            });
        }
    };

    return (
        <div id="App" style={{padding: "20px", fontFamily: "monospace"}}>
            <h1>data-agent</h1>
            <p>Data analysis tool — {cases.length} case(s)</p>
            <button onClick={handleCreate}>New Case</button>

            <h2>Cases</h2>
            {cases.length === 0 ? (
                <p>No cases yet.</p>
            ) : (
                <ul>
                    {cases.map(c => (
                        <li key={c.id}>{c.name} ({c.status})</li>
                    ))}
                </ul>
            )}

            <h2>Log</h2>
            <div style={{
                background: "#1a1a2e",
                color: "#0f0",
                padding: "10px",
                maxHeight: "200px",
                overflow: "auto",
                fontSize: "12px"
            }}>
                {logs.length === 0 ? (
                    <p>No log entries.</p>
                ) : (
                    logs.map((entry, i) => (
                        <div key={i}>[{entry.level}] {entry.message}</div>
                    ))
                )}
            </div>
        </div>
    );
}

export default App;
