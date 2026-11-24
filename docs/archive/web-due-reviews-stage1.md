# 初回の復習期限一覧

ブロック5 PR1時点の実装。現行の契約と実装は[復習期限一覧](../learning/web-due-reviews.md)を参照。

## 何のためか

ログインした本人が、いま初回復習できる記録を `GET /web/reviews/due` で取得する。たとえば11月24日16時35分（日本時間）に保存した記録は、11月27日16時35分ちょうどから一覧に入り、記録ID、`stage: 1`、`due_at` が返る。

## 状態の流れ

保存される状態の更新はない。初回期限に達したかをリクエストごとに判定する。

## 呼び出しの流れ

1. `cmd/server/main.go:main` → `learningrecord/handler_web.go:RegisterWebRoutes`。Webセッション認証と `CrossOriginProtection` の内側にルートを登録する。未ログインは401。
2. `learningrecord/handler_web.go:listDueReviews` → `learningrecord/review.go:Service.listDueReviews`。認証済みの本人IDを渡す。
3. `learningrecord/review.go:Service.listDueReviews` → `learningrecord/review.go:firstReviewCutoff`。時計を1回読み、現在から72時間前を求める。SQLは `user_id = $1 AND created_at <= $2` で、本人かつ保存から72時間以上経過した記録のIDと保存日時を読む。
4. `learningrecord/review.go:Service.listDueReviews` → `learningrecord/review.go:firstReviewDueAt`。SQLが古い順に返した各保存日時に72時間を加える。ハンドラはその順序で `records` を返す。0件なら空配列、読み取りに失敗したら500 `internal_error`。

## 関数ごとの表

| 関数 | 目的 | 入力 | 出力 | 更新するもの |
|---|---|---|---|---|
| `listDueReviews`（ハンドラ） | 本人の復習期限をJSONで返す | リクエスト、本人ID | 200と一覧、または500 | なし |
| `Service.listDueReviews` | 期限を迎えた本人の記録を読む | 本人ID、サービスの時計 | 記録ID、段階、期限の一覧、またはエラー | なし |
| `firstReviewCutoff` | 期限に達した最新の保存日時を求める | 現在時刻 | 現在時刻の72時間前 | なし |
| `firstReviewDueAt` | 初回の復習日時を求める | 保存日時 | 保存日時の72時間後 | なし |

## なぜそうしたか

- `created_at + 72時間 <= 現在` と同じ意味の `created_at <= 現在 - 72時間` で絞る。SQLの列に日時関数を掛けず、既存の所有者・保存日時のインデックスを使える。
- 一覧が必要とするIDと日時だけを読み、問題・回答・フィードバックは既存の詳細APIで取得できる。

## 未決

記録全体の復習結果を決める方法と保存契約、「一部できた」の再試行間隔は[仕様04](../specs/04-review.md#未決)に残る。結果保存と段階進行、画面での表示、メール通知は後続で扱う。
