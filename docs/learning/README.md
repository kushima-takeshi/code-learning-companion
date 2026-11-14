# 学習ノート

今のコードの振る舞いを、機能ごとに1ファイルで書く。節の順は [AGENTS.md](../../AGENTS.md) の「学習ノート」。用語は [用語集](glossary.md)。仕様は [07](../specs/07-agent-connection.md)。

| ノート | 目的 |
|---|---|
| [再接続の開始](reconnect-start.md) | 旧トークンから再接続を始める。使えない旧トークンの新規開始は拒否する |
| [再接続の状態取得](reconnect-status.md) | 再接続リクエストを、DBを変えずに表示する |
| [再接続の完了](reconnect-complete.md) | 確認済みの再接続で、旧トークンを失効する |
| [再接続のやり直し](reconnect-restart.md) | 期限内の未完了を捨て、置き換え先を発行し直す |
| [再接続の確認](reconnect-confirm.md) | 置き換え先の学習記録一覧の成功を、`confirmed` として残す |
| [接続一覧](token-list.md) | 本人の接続を返す。完了していない置き換え先は出さない |
| [接続画面](connection-screen.md) | ログイン後に接続を足し、見分け、失効する。原本は追加の直後だけ出す |
| [再接続の画面](reconnect-screen.md) | 使える接続の行の下で、再接続を始め、確認し、完了するかやり直す |
| [学習記録の一覧ツール](mcp-list.md) | stdioのMCPから、引数なしで学習記録の一覧を取る |
| [学習記録の保存ツール](mcp-save.md) | stdioのMCPから、確認済みの学習記録を保存する |

説明を足すときは、当たるノートの節へ織り込む。末尾へ経緯を足さない。今のコードと合わない説明は消さず `docs/archive/` へ移し、ここからリンクする。

分けた前の1本は [再接続の学習ノート（分割前）](../archive/learning-reconnect-before-split-2026-11-06.md)。[PR #33](https://github.com/kushima-takeshi/code-learning-companion/pull/33) で入り、PR #35 と PR #37 の説明が末尾に足されていた。履歴であり、今の振る舞いの正本ではない。
