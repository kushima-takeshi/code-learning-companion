# 詳細画面で復習結果を保存する

API契約は [仕様04](../specs/04-review.md)。画面の配置は [仮の画面イメージ](../mockups/block5-pr4-detail-save.png) を参照する。

## 何のためか

学習記録の詳細で問題・回答・解説を見返し、期限が来た復習の結果を記録する。たとえば「復習 2回目」で「できた」を押すと、2回目の結果を保存してホームへ戻る。次の期限は記録の保存日時から30日後のまま。すでにその期限も来ていれば、ホームに3回目が表示される。

## 状態の流れ

```mermaid
stateDiagram-v2
    [*] --> 取得中: 詳細と期限一覧を取得
    取得中 --> 結果を選べる: 詳細を取得し記録IDが一致
    取得中 --> 詳細のみ: 一覧に記録がない
    取得中 --> 詳細のみ: 期限一覧の取得失敗を表示
    取得中 --> GitHubログイン: 401
    結果を選べる --> 保存中: いずれかの結果を押す
    保存中 --> ホーム: 201、期限一覧を再取得
    保存中 --> GitHubログイン: 401
    保存中 --> 結果を選べる: 409またはその他の失敗を表示
```

詳細そのものの取得とエラー表示は [学習記録の詳細画面](learning-record-detail.md) を参照する。

## 呼び出しの流れ

1. `learning/LearningRecordDetailSection.tsx:loadRecord` と `learning/LearningRecordDetailSection.tsx:loadDueReview` が並行して詳細と期限一覧を読む。後者は `api/learningRecords.ts:listDueReviews` を呼び、パスのIDを `decodeURIComponent` して一覧の `id` と照合する。一致した行の `stage` と `due_at` を画面に使う。
2. `learning/LearningRecordDetailSection.tsx:handleSaveReview` → `api/learningRecords.ts:saveReviewResult`。Cookie付きの `POST /web/learning-records/{id}/reviews` に、選んだ `result` と表示中の `stage` を送る。
3. `internal/learningrecord/handler_review.go:saveReview` → `internal/learningrecord/review.go:Service.saveReview`。SQLは本人の記録で指定段階が未保存の最小段階に合うときに結果を挿入する。詳細は [復習結果の保存API](web-review-results.md) を参照する。
4. 201では `App.tsx:navigate` が `pushState` とパスの更新でホームを表示し、`learning/DueReviewsSection.tsx:loadDueReviews` が期限一覧を再取得する。401では `App.tsx:handleSessionExpired` を呼ぶ。

## 関数ごとの表

| 関数 | 目的 | 入力 | 出力 | 更新するもの |
|---|---|---|---|---|
| `loadDueReview` | 現在の復習を探す | パスのID、中断シグナル | なし | 一致した行または取得エラー、401時のログイン状態 |
| `handleSaveReview` | 選んだ結果を保存して応答を反映する | 選んだ結果、表示中の復習 | なし | ボタンの有効状態、保存エラー、表示中の画面 |
| `saveReviewResult` | 既存APIに結果と段階を送る | パスのID、結果と段階、中断シグナル | `saved`、`unauthorized`、`conflict`、またはエラー | APIを通じて復習結果を1件保存 |

## なぜそうしたか

- 期限と段階はサーバーの期限一覧から取得する。詳細の本文と別に取得するため、期限一覧の401以外の失敗でも本文を見返せる。
- 保存中は4つのボタンを無効にし、通信中の要求があればハンドラも送信を止める。画面を離れると通信の待機を中断する。詳細を開き直すと現在の段階を再取得する。
- 409 では詳細を開き直すよう案内する。別のタブなどで保存された後に、古い表示から次の段階を進めることを防ぐため、常に表示中の段階を送る。

## 未決

- 期限前に先行保存する導線と、3回分を保存したときの「復習計画完了」表示は後続で検討する。
