# data-agent

対話型チャットインターフェースを備えたデータ分析デスクトップGUIツール。

JSON/JSONL/CSV/TSV/SQLiteデータをケースごとに独立したDuckDBに取り込み、自然言語クエリまたは直接SQLで探索する。テーブル表示、グラフ可視化、Markdownレポート生成、コンテナベースのPython分析に対応。

## 機能

- **ケースベースのデータ管理** — 調査/プロジェクトごとにデータセットを分離
- **自然言語分析** — 知りたいことを記述するとLLMがSQLを生成・実行
- **直接SQLモード** — `/sql` で生SQLに切替
- **デュアルLLMバックエンド** — Vertex AI（Gemini）とローカルLLM（OpenAI互換API）
- **コンテナ実行** — Podman/Dockerサンドボックス内でPython分析コードを実行
- **複数出力形式** — テーブル、グラフ（棒/折れ線/円）、Markdownレポート

## インストール

[リリースページ](https://github.com/nlink-jp/data-agent/releases)からビルド済みバイナリをダウンロード。

## ビルド

```sh
make build    # macOSアプリをビルド → dist/data-agent.app
make dev      # ホットリロード付き開発モード
make test     # テスト実行
```

必要環境: Go 1.26+, Node.js, [Wails v2](https://wails.io/)

## 設定

設定は `~/Library/Application Support/data-agent/config.toml` に保存。

```toml
[vertex_ai]
project = "your-project-id"
region = "us-central1"
model = "gemini-2.5-flash"

[local_llm]
endpoint = "http://localhost:1234/v1"
model = "google/gemma-4-26b-a4b"
api_key = ""

[container]
runtime = "podman"
```

## ライセンス

MIT — [LICENSE](LICENSE) を参照。
