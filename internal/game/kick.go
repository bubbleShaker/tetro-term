package game

// noKick は「ズラさない」候補ひとつだけの表。
var noKick = []Point{{X: 0, Y: 0}}

// kicks は from から to への回転で試すオフセットの列を、試す順に返す
// （→ CONTEXT.md「キックテーブル」）。
//
// M1 の中身は空っぽ同然だが、それでよい。回転処理の側は「候補を順に試す」形だけを持ち、
// 候補の中身はこのデータが決める（→ docs/adr/0002）。M6 で SRS の表を流し込むとき、
// 書き換わるのはこの関数だけで、rotate には手が入らない。
//
// 引数は使っていないが消さない。SRS の表は「ミノの種類 × どの向きからどの向きへ」で
// 引くものなので、その形をいまのうちに固定しておく。
func kicks(kind MinoKind, from, to Rotation) []Point {
	_, _, _ = kind, from, to
	return noKick
}
