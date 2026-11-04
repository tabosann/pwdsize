package main

import(
	"os"
	"fmt"
	"math"
	"sort"
	"sync"
	"strings"
	"path/filepath"
)

type FileSize struct {
	path string
	size int64
	isDir bool
	sizeCh <-chan int64
}

func generateDirSizeRec(
	quit <-chan int,
	sizes chan int64,
	dir string,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	size := int64(0)
	for _, file := range files {
		if file.IsDir() {
			fpath := filepath.Join(dir, file.Name())
			wg.Add(1)
			go generateDirSizeRec(quit, sizes, fpath, wg)
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		size += info.Size()
	}
	sizes <- size
}

func generateSizes(quit <-chan int, dir string) <-chan int64 {
	sizes := make(chan int64)
	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Go(func () {
		defer wg.Done()

		files, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		size := int64(0)
		for _, file := range files {
			if file.IsDir() {
				fpath := filepath.Join(dir, file.Name())
				wg.Add(1)
				go generateDirSizeRec(quit, sizes, fpath, &wg)
				continue
			}

			info, err := file.Info()
			if err != nil {
				continue
			}

			size += info.Size()
		}
		sizes <- size
	})

	go func() {
		// 全ゴルーチンが完了してから閉じる
		wg.Wait()
		close(sizes)
	}()

	return sizes
}

func sortSize(quit <-chan int, sizeList ...FileSize) <-chan string {
	output := make(chan string)
	go func() {
		defer close(output)

		wg := sync.WaitGroup{}
		for i, e := range sizeList {
			if !e.isDir {
				// ファイルサイズはすでに分かっているのでスキップ
				continue
			}

			// ディレクトリサイズの算出に入る
			wg.Add(1)
			wg.Go(func () {
				defer wg.Done()

				var size int64
				ok := true
				for ok {
					select {
					case size, ok = <-sizeList[i].sizeCh:
						if ok {
							sizeList[i].size += size
						}
					case <-quit:
						ok = false
					}
				}
			})
		}
		wg.Wait()

		// サイズの昇順にソート
		sort.Slice(sizeList, func(a, b int) bool {
			return sizeList[a].size > sizeList[b].size
		})

		strList := make([]string, len(sizeList))
		for i, e := range sizeList {
			if e.isDir {
				strList[i] = fmt.Sprintf("%s * %s\\", humanSize(e.size), e.path)
			} else {
				strList[i] = fmt.Sprintf("%s   %s", humanSize(e.size), e.path)
			}
		}

		output <- strings.Join(strList, "\n")
	}()
	return output
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%8.3f   B", float64(bytes))
	}

	// 単位の定義
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	
	// 適切な単位のインデックスを計算
	// エグザバイト(EiB)以上を丸める安全策
	exp := int(math.Log(float64(bytes)) / math.Log(float64(unit)))
	exp = min(exp, len(units))

	value := float64(bytes) / math.Pow(float64(unit), float64(exp))

	return fmt.Sprintf("%8.3f %s", value, units[exp-1])
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("%s <root dir>\n", os.Args[0])
		os.Exit(1)
	}


	root := os.Args[1]
	files, err := os.ReadDir(root)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	sizeList := make([]FileSize, 0)
	quit := make(chan int)

	for _, file := range files {
		fpath := filepath.Join(root, file.Name())

		if file.IsDir() {
			sizeList = append(sizeList, FileSize{
				path: fpath,
				size: int64(0),
				isDir: true,
				sizeCh: generateSizes(quit, fpath),
			})

			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		sizeList = append(sizeList, FileSize{
			path: fpath,
			size: info.Size(),
			isDir: false,
			sizeCh: nil,
		})
	}

	result := sortSize(quit, sizeList...)
	fmt.Println(<-result)
}
