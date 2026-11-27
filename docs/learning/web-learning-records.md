# Webセッションで学習記録を読む

契約は [仕様03のブロック4](../specs/03-learning-questions.md#アプリ画面用の学習記録api)。セッション認証の型は [仕様07のWebトークン管理API](../specs/07-agent-connection.md#webトークン管理api2026-10-31-実装検証済み) と共通。

## 何のためか

Webにログインした本人が、保存した学習記録をアプリで見返せるようにする。たとえば `GET /web/learning-records?limit=1` で最新の記録を探し、返ったIDを `GET /web/learning-records/{id}` に渡して問題・回答・フィードバックと復習状態を読む。どちらも、リポジトリ名が保存されていれば `repository` を返す。

## 状態の流れ

状態の更新はない。一覧・詳細とも、セッションと学習記録を読む。詳細では保存済みの復習結果も読む。

## 呼び出しの流れ

1. `cmd/server/main.go:main` → `learningrecord/handler_web.go:RegisterWebRoutes`。Web用パスをセッション認証と `CrossOriginProtection` の内側に登録する。
2. `auth/middleware.go:RequireAPIAuth` → `auth/middleware.go:requireAuth` → `auth/store.go:FindUserByTokenHash`。セッションCookieをハッシュ化し、有効期限内のセッションの本人をSQLで取得する。本人をリクエストのコンテキストへ入れる。
3. 一覧は `learningrecord/handler_web.go:listWeb` → `learningrecord/handler.go:listPage` → `learningrecord/list.go:List`。SQLは `WHERE user_id = $1` で絞り、`created_at DESC, id DESC` で最大 `limit + 1` 件を読む。余分な1件で `has_more` を決める。
4. 詳細は `learningrecord/handler_get.go:getWeb` → 同ファイルの `getRecord` → `learningrecord/query.go:Get`。パスのUUIDを正規化し、SQLは `WHERE id = $1 AND user_id = $2` で読む。行がなければ404になる。
5. `learningrecord/handler_get.go:getWeb` → `learningrecord/review.go:getReviewStatus`。本人の1件について、`LEFT JOIN LATERAL` で `nextReviewStage` の結果を取り、`reviewDueAt` で期限を求める。次の段階があれば `due_at <= s.now()` の結果も返し、段階がなければ `completed: true` だけを返す。`getWeb` が `learningrecord/handler_record.go:publicRecord` の記録に `review` を加えて応答する。

## 関数ごとの表

| 関数 | 目的 | 入力 | 出力 | 更新するもの |
|---|---|---|---|---|
| `RegisterWebRoutes` | Web用の学習記録・復習の入口を登録 | mux、認証ミドルウェア、信頼するOrigin | 登録エラーまたはnil | muxのルート |
| `RequireAPIAuth` | 共通のセッション照合を使い、DB障害をJSONで通知 | session Cookie、次のハンドラ | 本人を含むリクエスト、または401・500 | リクエストのコンテキスト |
| `FindUserByTokenHash` | セッションから本人を調べる | Cookieのハッシュ | 有効なセッションのユーザー、またはエラー | なし |
| `listWeb` / `listPage` | ページ指定を検証し、本人の一覧を返す | 本人ID、`limit`、`offset` | `ListPage` のJSON、または400・500 | なし |
| `Service.List` | SQLで本人の一覧を読む | 本人ID、件数、開始位置 | `RecordSummary` の配列と `has_more`、またはエラー | なし |
| `getWeb` | Webの詳細応答を組み立てる | 本人ID、HTTPリクエスト | 記録と `review` のJSON、または400・404・500 | なし |
| `getRecord` | UUIDを検証し本人の記録を読む | 本人ID、パスのID | `Record` と取得の成否。失敗時は400・404・500を応答 | なし |
| `Service.Get` | SQLで本人の1件を読む | 本人ID、正規化した記録ID | `Record` またはエラー | なし |
| `Service.getReviewStatus` | SQLで現在の復習を求める | 本人ID、記録ID、サーバー時刻 | 段階・期限・到来の有無、完了、またはエラー | なし |

## なぜそうしたか

- パスを分けることで、ブラウザとAIツールが認証方式をURLで選べる。両方の認証情報が届いても、Webはセッション、AIツール用APIは連携トークンで所有者を決める。
- 所有者をSQLの条件に含めると、他人のIDと存在しないIDはどちらも0件になり、同じ404を返せる。
- ページ分割と詳細のUUID検証は両方の入口で共有する。Web 詳細の書き出しでだけ `review` を加えることで、Bearer 詳細と学習記録の保存応答を同じ形式で使える。
- 期限一覧と同じ `nextReviewStage` と `reviewDueAt` を使い、段階・期限のルールを一か所で管理する。`LEFT JOIN` は全段階を終えた記録も残すために使う。
- サーバーが `due` を返すため、端末とサーバーの時計差で期限ちょうどの判定がずれない。
- 認証エラーにも `Cache-Control: no-store` を付けるため、認証より外側でヘッダーを設定する。

## 未決

このAPIに未決の契約はない。画面の挙動は [履歴一覧](learning-history-list.md) と [学習記録の詳細](learning-record-detail.md) を参照する。
