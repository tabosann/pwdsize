package main

import(
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"math"
	"sort"
)

type dirSizeMtx struct {
	mtx sync.Mutex
	size int64
}

// マップの値をソートする際に必要
type mapPair struct {
	key string
	val int64
}

// マップの値を降順でソートするインタフェース
type sortMapValD []mapPair
func (p sortMapValD) Len() int {
	return len(p)

}
func (p sortMapValD) Less(i, j int) bool {
	return p[i].val > p[j].val

}
func (p sortMapValD) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("%s <root dir>\n", os.Args[0])
		os.Exit(1)
	}

	root := os.Args[1]
	files, err := os.ReadDir(root)
	if err != nil {
		fmt.Println("Not found", root)
		os.Exit(1)
	}

	wg := sync.WaitGroup{}
	fsizes := make(map[string]int64)
	dsizes := make(map[string]*dirSizeMtx)
	for _, file := range files {
		fpath := filepath.Join(root, file.Name())

		if file.IsDir() {
			dsizes[fpath] = &dirSizeMtx{
				mtx: sync.Mutex{},
				size: 0,
			}
			wg.Add(1)
			go calcDirSize(dsizes[fpath], fpath, &wg)
			continue
		}

		info, err := file.Info()
		if err != nil {
			fmt.Println("Failed to get file info", fpath)
			os.Exit(1)
		}

		fsizes[fpath] += info.Size()
	}

	wg.Wait()

	i := 0
	pairs := make(sortMapValD, len(fsizes) + len(dsizes))
	for k, v := range fsizes {
		pairs[i] = mapPair{ fmt.Sprintf("  %s", k), v }
		i++
	}
	for k, v := range dsizes {
		pairs[i] = mapPair{ fmt.Sprintf("* %s\\", k), v.size }
		i++
	}

	sort.Sort(pairs)

	for _, v := range pairs {
		fmt.Printf("%s %s\n", humanSize(v.val), v.key)
	}
}

func calcDirSize(ret *dirSizeMtx, dir string, wg *sync.WaitGroup) {
	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println("Not found", dir)
		os.Exit(1)
	}

	ret.mtx.Lock()
	for _, file := range files {
		fpath := filepath.Join(dir, file.Name())

		if file.IsDir() {
			wg.Add(1)
			go calcDirSize(ret, fpath, wg)
			continue
		}

		info, err := file.Info() 
		if err != nil {
			fmt.Println("Failed to get file info", fpath)
			os.Exit(1)
		}

		ret.size += info.Size()
	}
	ret.mtx.Unlock()

	wg.Done()
}

// humanSize はバイトサイズを人間が見やすい文字列に変換します。
func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%8.3f   B", float64(bytes))
	}

	// 単位の定義
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	
	// 適切な単位のインデックスを計算
	exp := int(math.Log(float64(bytes)) / math.Log(float64(unit)))
	if exp > len(units) {
		exp = len(units) // エグザバイト(EiB)以上を丸める安全策
	}

	value := float64(bytes) / math.Pow(float64(unit), float64(exp))

	return fmt.Sprintf("%8.3f %s", value, units[exp-1])
}
