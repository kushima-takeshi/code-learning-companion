# ドキュメント案内

更新日: 2026-10-26

現行方針は第二案（2026-10-26に公開PR必須の前提を撤回）。学習は手元のコード・差分・許可された会話から、ユーザーのAIツール内の自作スキルで行う。
Learning Companionは、送信内容を確認した問題・回答・フィードバック等の学習記録を保存し、閲覧・復習予定・通知を担当する。元教材のアプリ保存は必須にせず、既存の公開PR取り込みは任意の入口として残す。

## 読む順序と役割

| 文書 | 用途 |
|---|---|
| [伴走ルール](../AGENTS.md) | AIの読む順序、ユーザーが実装する進め方 |
| [現在の作業](current-work.md) | 今どこまでできたか、次の一手、未決事項。過去の日付付き記録は archive |
| [要件定義](../requirements.md) | 何を実現するか、対象外 |
| [第二案・現行作業計画](implementation-plan-v2.md) | どの順番で作るか |
| [設計概要](detailed-design.md) | 責任分担、共通データ、仕様への入口 |
| [仕様01](specs/01-authentication.md)・[仕様02](specs/02-public-pr-import.md) | 既存のWeb認証と任意の公開PR取り込み |
| [仕様03](specs/03-learning-questions.md) | スキルによる出題・回答・判定と保存・再開 |
| [仕様04](specs/04-review.md) | 復習・通知 |
| [仕様05](specs/05-deep-dive.md) | AIツール内の深掘りと学習要点 |
| [仕様06](specs/06-data-lifecycle.md) | 保持・削除・上限・公開条件 |
| [仕様07](specs/07-agent-connection.md) | AIツールの接続認証・連携トークン |
| [学習ノート](learning/README.md) | 機能ごとの今の振る舞い。用語は [用語集](learning/glossary.md)。節の順は [伴走ルール](../AGENTS.md) |

仕様は完成時の条件であって、実装済みの保証ではない。実装状況は現在の作業とコードで確認する。
第二案の全提案値が確定したわけではない。未決事項は現在の作業へ集約し、詳細仕様の未決箇所を旧案で埋めない。

## 履歴・参考資料（実装の正本ではない）

- [第一案](implementation-plan.md): 旧作業計画。フェーズ2の未決課題一覧もここに残る。
- [旧フェーズ3設計](phase3-design.md): BYOK方式の旧草案。API・状態の定義を流用して実装しない。
- [AI連携の検討経緯](ai-tool-integration-study.md): 収集方式などの調査履歴。後の方針は現行文書を参照。
- [改訂前の文書一式](archive/2026-10-26-before-v2/README.md): 今回書き換えた要件・設計・仕様の原本。
- [現在の作業の記録（2026-10）](archive/current-work-2026-10.md): `current-work.md` から移した2026-10の完了記録。
- [現在の作業の記録（2026-11）](archive/current-work-2026-11.md): `current-work.md` から移した2026-11の完了記録。
- [再接続の学習ノート（分割前）](archive/learning-reconnect-before-split-2026-11-06.md): 機能ごとに分ける前の `docs/learning/reconnect.md`。今の説明は [学習ノート](learning/README.md)。
- [将来案](../future-ideas.md): 現在の必須要件ではない。

## 文書を更新するとき

目的・範囲の変更は要件、振る舞いの変更は対応仕様、順序の変更は作業計画、実装進捗は現在の作業に記録する。
関連する現行文書も整合させる。履歴資料を現行仕様へ戻さない。
合意が振る舞いを変えるときは、対応する仕様の項目のそばに判断(日付)・理由・却下案を1行で添える。月の作業が終わったら、`current-work.md` のその月の日付付き節を `docs/archive/current-work-YYYY-MM.md` へ移す。
