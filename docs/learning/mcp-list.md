# 学習記録の一覧ツール

仕様は [08](../specs/08-local-mcp.md)。このノートは `list_learning_records` だけ。保存は [学習記録の保存ツール](mcp-save.md)。詳細取得は [学習記録の詳細取得ツール](mcp-get.md)。

## 何のためか

使う人は、手元のAIツール。入力は任意の `limit` と `offset`。出力は本人の学習記録の `id`、`theme`、`created_at` と `has_more`。問題本文は返さない。データベースはこのプロセスでは更新しない。

## 状態の流れ

状態は無い。呼ぶたびに `GET /learning-records` を1回送る。

## 呼び出しの流れ

stdio。待ち受けるポートは無い。起動は [main.go](../../mcp/cmd/adapter/main.go)。

| 順 | 呼び出し | SQL |
|---|---|---|
| 1 | [config.go Load](../../mcp/internal/config/config.go) | なし |
| 2 | [client.go New](../../mcp/internal/api/client.go) | なし |
| 3 | [server.go New](../../mcp/internal/server/server.go) | なし。一覧ツールを登録する |
| 4 | [server.go listLearningRecords](../../mcp/internal/server/server.go) | なし。`DoJSON` で GET |

## 関数ごとの表

| 関数 | 目的 | 入力 | 出力 | 更新するもの |
|---|---|---|---|---|
| `Load` | トークンと基点を読む | 環境変数 | `Config` かエラー | なし |
| `New`（client） | HTTPクライアントを作る | 基点、トークン | `*api.Client` | なし |
| `New`（server） | MCPサーバーを作る | `*api.Client` | `*mcp.Server` | なし |
| `listLearningRecords` | 一覧を取る | context、client、`listInput` | 一覧かツールエラー | なし |
| `listPath` | 指定された項目だけをクエリにする | `listInput` | パス | なし |

## なぜそうしたか

`limit` と `offset` は、値が入っているときだけクエリに載せる。空ならクエリは無い。APIの既定は `limit` 20、`offset` 0、`limit` は1〜100。範囲外はAPIが400 `invalid_pagination` にする。失敗はステータスと `error` のコードだけを返し、応答本文は返さない。

## 未決

保存は [学習記録の保存ツール](mcp-save.md)。詳細取得は [学習記録の詳細取得ツール](mcp-get.md)。実プロセスでの確認は未了。Issue #44 と全体ブロック2は未完了。
