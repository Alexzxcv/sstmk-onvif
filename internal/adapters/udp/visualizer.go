package udp

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

func generateZoneImage(zones *DetectorZones) ([]byte, error) {
	const (
		rows   = 6
		cols   = 2
		cellW  = 50 // Ширина одной клетки в пикселях
		cellH  = 50 // Высота одной клетки
		border = 2  // Толщина границы между клетками
		width  = cols * cellW
		height = rows * cellH
	)

	// Создаем холст (RGBA)
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Цвета
	colActive := color.RGBA{255, 0, 0, 255}       // Красный (активная зона)
	colInactive := color.RGBA{220, 220, 220, 255} // Серый (пустая зона)
	colBorder := color.RGBA{0, 0, 0, 255}         // Черный (границы)

	// 1. Заливаем всё черным цветом (это будут границы)
	draw.Draw(img, img.Bounds(), &image.Uniform{colBorder}, image.Point{}, draw.Src)

	// 2. Рисуем клетки
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			// Логика координат:
			// r=0 — это низ детектора. В изображении Y=0 — это верх.
			// Поэтому переворачиваем по вертикали: (rows - 1 - r)
			y := (rows - 1 - r) * cellH
			x := c * cellW

			// Выбираем цвет
			fillColor := colInactive
			// Проверка: Type != 0 значит зона активна
			if zones.Alarm[r][c].Type != 0 {
				fillColor = colActive
			}

			// Рисуем внутренний квадрат, оставляя рамку (border)
			rect := image.Rect(
				x+border,
				y+border,
				x+cellW-border,
				y+cellH-border,
			)
			draw.Draw(img, rect, &image.Uniform{fillColor}, image.Point{}, draw.Src)
		}
	}

	// 3. Кодируем в PNG (или JPEG для скорости, если нужно)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
