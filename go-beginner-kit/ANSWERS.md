# 解答と考え方

完全な解答コードは`answers/exercises.go`、テストは`answers/exercises_test.go`にあります。

```sh
go test ./answers -v
```

このコマンドは教材の模範解答を確認するものです。自分の解答は`practice`に作り、`go test ./practice -v`で確認します。

## E1：条件が確定したら返す

```go
func Level(minutes int) int {
    if minutes <= 0 {
        return 0
    }
    if minutes < 60 {
        return 1
    }
    return 2
}
```

2つ目のifまで到達した時点で、minutesが1以上なのは確定しています。重ねて`minutes >= 1`と書く必要はありません。

## E2：空入力を先に除外し、割る前に変換する

```go
func Average(values []int) (float64, error) {
    if len(values) == 0 {
        return 0, errors.New("values must not be empty")
    }
    total := 0
    for _, value := range values {
        total += value
    }
    return float64(total) / float64(len(values)), nil
}
```

`errors`のimportが必要です。今回の仕様では、空入力を平均0と扱うと「0分の平均」と区別できないため、errorを返します。

## E3：集計先を新しく作る

```go
func Totals(records []Record) map[string]int {
    result := make(map[string]int)
    for _, record := range records {
        result[strings.TrimSpace(record.Title)] += record.Minutes
    }
    return result
}
```

`strings`のimportが必要です。元のRecordへ代入していないため、入力タイトルの空白はそのまま残り、集計先のキーだけが正規化されます。

## E4：検証してから更新する

```go
func (r *Record) SetMinutes(minutes int) error {
    if minutes < 1 || minutes > 1440 {
        return errors.New("minutes must be between 1 and 1440")
    }
    r.Minutes = minutes
    return nil
}
```

呼び出し元のRecordを更新するのでポインタレシーバです。不正入力では代入まで到達しません。

## E5：合計処理は保存方法を知らなくてよい

```go
func TotalFrom(reader Reader) int {
    total := 0
    for _, record := range reader.List() {
        total += record.Minutes
    }
    return total
}

type fakeReader struct{}

func (fakeReader) List() []Record {
    return []Record{
        {Title: "Go", Minutes: 20},
        {Title: "SQL", Minutes: 30},
    }
}
```

`TotalFrom(fakeReader{})`は50です。テスト用の型は必要なメソッドだけを実装できます。

## 理解確認の解答

| 問い | 解答 |
|---|---|
| C1 | 新しい変数を宣言して値を入れるときは`:=`、既存変数への再代入は`=`。同じスコープの短い変数宣言には少なくとも1つ新しい変数が必要。 |
| C2 | errorがある場合、intは有効な結果とは限らないため。この教材では失敗時の仮の値として0を返している。 |
| C3 | スライスの情報がコピーされても、要素を保存する内部配列は共有されているため。 |
| C4 | `value, ok := m[key]`のokを見る。falseなら未登録。 |
| C5 | Record値ならコピーのフィールドが変わる。*Recordなら指している元のRecordのフィールドを変えられる。 |
| C6 | Goにはimplements宣言がなく、必要なメソッドを持つことでinterfaceを満たす。 |
| C7 | Unmarshalが呼び出し元のrecordに書き込めるよう、ポインタを渡すため。 |
| C8 | `C 2`、`B 2`、`A 1`。通常の表示が先。deferは後に登録したものからで、引数は登録時に評価される。 |
| C9 | abcはIDとして形式不正なので400。999はIDの形式は正しいが対象が存在しないので404。 |
| C10 | キャンセルは合図であり、強制停止ではない。処理側が合図を確認するか、context対応の処理へ渡す必要がある。 |

## API最終課題の期待結果

| 操作 | 期待結果 |
|---|---|
| 新規起動後の一覧 | 200、`[]` |
| Go / 25分を登録 | 201、ID1 |
| SQL / 30分を登録 | 201、ID2 |
| 一覧 | 200、ID1・ID2の順に2件 |
| ID1詳細 | 200、Go / 25分 |
| 空タイトル・分数0・1441 | 400、登録されない |
| 分数が文字列 | JSONをstructへ読み込む段階で400 |
| `/records/abc` | 400 |
| `/records/999` | 404 |
| 不正入力の後の一覧 | 2件のまま |
| 再起動後の一覧 | 200、`[]`。メモリ保存のため消える |

タイトルや分数のルールは`study.Validate`、保存前の呼び出しは`Store.Add`、HTTPステータスへの変換は`handlers.go`です。JSONの型が合わない場合は、Validateより前のDecodeで失敗します。

## 復習で確認したいこと

コードの細部を暗記する必要はありません。次の3つを言葉で説明できるか確かめてください。

- なぜそのデータをコピーするのか、または共有するのか。
- どの失敗をどの戻り値で知らせるのか。
- 関数の入力と期待結果を、テストでどう表現するのか。
