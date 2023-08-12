package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	buffersSize   int           = 10               //размер буфера
	timeBeforeDel time.Duration = 10 * time.Second //время ожидания до извлечения данных из буфера
)

// структура данных кольцевого буфера
type IntRingBuffer struct {
	data   []int
	end    int
	size   int
	access sync.Mutex
}

// функция - конструктор для создания нового экземпляра буфера
func NewRingBuffer(size int) *IntRingBuffer {
	return &IntRingBuffer{make([]int, buffersSize), -1, size, sync.Mutex{}}
}

// Метод для добавления данных в кольцевой буфер. При превышении заданного объема будет стирать самые старые данные.
func (r *IntRingBuffer) Push(digit int) {
	r.access.Lock()
	if r.end == r.size-1 {
		for i := 1; i < r.size; i++ {
			r.data[i-1] = r.data[i]
		}
		r.data[r.end] = digit
	} else {
		r.end++
		r.data[r.end] = digit
	}
	fmt.Println("Данные добавлены в буфер")
	r.access.Unlock()
}

// Метод для извления данных из буфера через заданный промежуток времени.
func (r *IntRingBuffer) Pop() []int {
	r.access.Lock()
	if r.end < 0 {
		r.access.Unlock()
		return nil
	}
	filteredData := r.data[:r.end+1]
	r.end = -1
	fmt.Println("Данные извлечены из буфера")
	r.access.Unlock()
	return filteredData
}

func main() {
	//функция - источник данных для пайплайна. Данные берутся из консоли.
	data := func() (<-chan int, <-chan bool) {
		dataSourse := make(chan int)
		done := make(chan bool)
		go func() {
			fmt.Println("Введите в консоль целое число для обработки пайплайном или exit для завершения работы программы.")
			scan := bufio.NewScanner(os.Stdin)
			var d string
			for {
				scan.Scan()
				d = scan.Text()
				if strings.EqualFold(d, "exit") {
					fmt.Println("Завершение работы.")
					close(done)
					return
				}
				in, err := strconv.Atoi(d)
				if err != nil {
					fmt.Println("Пайплайн работает только с целыми числами.")
					continue
				}
				dataSourse <- in
				fmt.Println("Данные считаны из консоли")
			}
		}()
		return dataSourse, done
	}

	//Функция - стадия пайплайна для отфильтровки отрицательных чисел.
	negativeFilter := func(dataSourse <-chan int, done <-chan bool) <-chan int {
		notNegativeData := make(chan int)
		go func() {
			for {
				select {
				case data := <-dataSourse:
					if data >= 0 {
						notNegativeData <- data
					}
				case <-done:
					return
				}
			}
		}()
		fmt.Println("Отфильтрованы негативные числа")
		return notNegativeData
	}

	//Функция - стадия пайплайна для отфильтровки 0 и чисел не кратных трем.
	multiple3Filter := func(notNegativeData <-chan int, done <-chan bool) <-chan int {
		notMultiple3Data := make(chan int)
		go func() {
			for {
				select {
				case data := <-notNegativeData:
					if (data != 0) && (data%3 == 0) {
						notMultiple3Data <- data
					}
				case <-done:
					return
				}
			}
		}()
		fmt.Println("Отфильтрованы числа равные 0 и кратные 3-м")
		return notMultiple3Data
	}

	//Функция - стадия буфферизации отфильтрованных данных.
	bufferStage := func(notMultiple3Data <-chan int, done <-chan bool) <-chan int {
		bufferChan := make(chan int)
		buffer := NewRingBuffer(buffersSize)
		go func() {
			for {
				select {
				case data := <-notMultiple3Data:
					buffer.Push(data)
				case <-done:
					return
				}
			}
		}()

		//Вспомогательная горутина для извлечения данных из буфера.
		go func() {
			for {
				select {
				case <-time.After(timeBeforeDel):
					dataFromBuffer := buffer.Pop()
					if dataFromBuffer != nil {
						for i := 0; i < len(dataFromBuffer); i++ {
							bufferChan <- dataFromBuffer[i]
						}
					}
				case <-done:
					return
				}
			}
		}()
		return bufferChan
	}

	//Функция - потребитель данных из пайплайна. Выводит построчно отфильтрованные данные.
	consumer := func(bufferChan <-chan int, done <-chan bool) {
		for {
			select {
			case data := <-bufferChan:
				fmt.Println("Отфильтрованные данные из буфера: ", data)
			case <-done:
				return
			}
		}
	}

	sourse, done := data()
	pipeline := bufferStage(multiple3Filter(negativeFilter(sourse, done), done), done)
	consumer(pipeline, done)
}
