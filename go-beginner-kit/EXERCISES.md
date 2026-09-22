# 練習問題

まず問題を読み、期待結果を自分で書き出してから実装してください。ヒントを見ても進まなければ`ANSWERS.md`、最後に`answers/exercises.go`を参照します。

## 自分の解答を置く場所

教材フォルダで`practice`フォルダを作り、その中に`exercises.go`を作成します。最初の行は`package practice`です。以下の宣言を必要に応じて加えます。

```go
package practice

type Record struct {
    Title   string
    Minutes int
}

type Reader interface {
    List() []Record
}
```

各問題の関数をこのファイルに追加します。第8章まで進んだら、`answers/exercises_test.go`を`practice/exercises_test.go`へコピーし、先頭だけ`package practice`へ変更して利用できます。5問が実装できたら`go test ./practice -v`を実行します。部分的に進める場合は、未実装の関数を参照するテストをまだコピーしないでください。

## E1：学習時間からレベルを返す（第2章）

次の関数を書いてください。

```go
func Level(minutes int) int
```

| 入力 | 戻り値 |
|---|---:|
| 0以下 | 0 |
| 1〜59 | 1 |
| 60以上 | 2 |

例：`Level(-1)`は0、`Level(30)`は1、`Level(60)`は2。

ヒント：小さい値から判定し、結果が決まったらreturnします。`0 / 1`と`59 / 60`の境界で確認してください。

## E2：平均時間を返す（第3〜4章）

```go
func Average(values []int) (float64, error)
```

入力は学習時間の配列です。この問題では各値は0〜1440、要素数は十分小さいと仮定し、各値の検証や整数オーバーフロー対応は不要です。

- 要素があれば平均とnilを返す。
- 要素数0なら0とerrorを返す。
- `Average([]int{10, 11})`は`10.5, nil`。
- `Average(nil)`も要素数0として扱う。

ヒント：`float64(total / len(values))`だと、整数の割り算をした後で変換してしまいます。

## E3：タイトルごとの学習時間を合計する（第4章）

```go
func Totals(records []Record) map[string]int
```

前後の空白を除去したタイトルをキーにして、時間を合計してください。入力済みのRecordは変更しません。この問題では、空白除去後のタイトルは空ではなく、分数は有効と仮定します。

入力：

```go
[]Record{
    {Title: " Go ", Minutes: 20},
    {Title: "SQL", Minutes: 10},
    {Title: "Go", Minutes: 30},
}
```

期待する内容：`Go`は50、`SQL`は10。空入力では空のmapを返してください。mapの表示順で正誤を判定してはいけません。

ヒント：`strings.TrimSpace`、`make(map[string]int)`、`+=`を使います。

## E4：不正な更新からデータを守る（第5章）

```go
func (r *Record) SetMinutes(minutes int) error
```

- 1〜1440ならr.Minutesを更新してnilを返す。
- 範囲外ならerrorを返し、既存値を変えない。
- rはnilではないと仮定する。

最初の値が25の場合、`SetMinutes(0)`後も25、`SetMinutes(30)`後は30になることを確認してください。

ヒント：代入する前に入力チェックします。なぜポインタレシーバなのかも説明してください。

## E5：interface越しに合計を求める（第6章）

```go
func TotalFrom(reader Reader) int
```

`reader.List()`が返す全RecordのMinutesを合計します。Recordの分数は有効で、readerはnilではないと仮定します。空一覧なら0を返します。

テスト用の`fakeReader`を作り、20分と30分のRecordを返してください。TotalFromの結果は50です。DBやHTTPサーバーを使わずに確認します。

ヒント：`fakeReader`に`List() []Record`を実装するだけでReaderとして渡せます。`implements`は不要です。

## 理解確認：説明できるか（C1〜C10）

1. **C1**：既にある変数の更新で`:=`と`=`をどう使い分けますか。
2. **C2**：`(int, error)`を返す関数で、なぜintを使う前にerrorを確認しますか。
3. **C3**：`b := a`でスライスを代入した後、`b[0]`を変えるとaにも見えるのはなぜですか。
4. **C4**：`map[string]int`で、未登録と登録済みの0をどう区別しますか。
5. **C5**：`func update(r Record)`と`func update(r *Record)`で、フィールド更新の結果はどう違いますか。
6. **C6**：GoのinterfaceとJavaのinterfaceで、実装宣言にどんな違いがありますか。
7. **C7**：`json.Unmarshal(data, &record)`で、なぜ`&`が必要ですか。
8. **C8**：次の関数は何を、どの順に表示しますか。

```go
func demo() {
    n := 1
    defer fmt.Println("A", n)
    n = 2
    defer fmt.Println("B", n)
    fmt.Println("C", n)
}
```

9. **C9**：`GET /records/abc`と`GET /records/999`で、なぜステータスを分けますか。ID999は未登録とします。
10. **C10**：`cancel()`を呼んでも、goroutineが必ずすぐに止まるとは限らないのはなぜですか。

## 最終課題：APIの動作確認

第9章のAPIを使い、次を順番に確認します。期待結果は`ANSWERS.md`にあります。

1. サーバーを新しく起動する。
2. 一覧を取得する。
3. `Go / 25分`と`SQL / 30分`を登録する。
4. 一覧とID1の詳細を取得する。
5. タイトルが空、分数0、分数1441、分数が文字列の入力を送る。
6. `/records/abc`と`/records/999`を取得する。
7. 不正入力によって一覧の件数が増えていないことを確認する。
8. サーバーを終了して再起動し、一覧を取得する。

合格の目安は「全部動いた」だけではありません。入力エラーがどの関数で判定され、どこでHTTPステータスになったかをコード上で指せれば合格です。
