package main

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"os/exec"
	"time"

	"gocv.io/x/gocv"
)

func main() {
	videoPath := "/home/x/PROYECTOS/Proyectos-Go/dog-counter/trafico_convertido.avi"
	width, height := 640, 360

	// Iniciar FFmpeg para lectura de video
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-vf", fmt.Sprintf("scale=%d:%d", width, height), "-f", "image2pipe", "-pix_fmt", "bgr24", "-vcodec", "rawvideo", "-")
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Error al conectar con la tubería de FFmpeg: %v", err)
	}

	// Capturar errores de FFmpeg
	stderr, err := cmd.StderrPipe()
	if err == nil {
		go func() {
			bufErr := make([]byte, 1024)
			for {
				n, err := stderr.Read(bufErr)
				if n > 0 {
					log.Printf("[FFmpeg] %s", string(bufErr[:n]))
				}
				if err != nil {
					break
				}
			}
		}()
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("Error al iniciar FFmpeg: %v", err)
	}
	defer cmd.Wait()

	window := gocv.NewWindow("Monitor de Tráfico - Autos Blancos")
	defer window.Close()

	mog2 := gocv.NewBackgroundSubtractorMOG2()
	defer mog2.Close()

	safeArea := image.Rect(100, 100, 540, 260)
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))
	defer kernel.Close()

	fgMask := gocv.NewMat()
	defer fgMask.Close()

	frameSize := width * height * 3
	buf := make([]byte, frameSize)

	fmt.Println("Iniciando monitoreo de autos blancos...")

	for {
		_, err := io.ReadFull(stdout, buf)
		if err != nil {
			log.Println("Fin del video o error de lectura de bytes.")
			break
		}

		frame, err := gocv.NewMatFromBytes(height, width, gocv.MatTypeCV8UC3, buf)
		if err != nil {
			log.Printf("Error al crear Mat desde bytes: %v", err)
			continue
		}

		if frame.Empty() {
			frame.Close()
			continue
		}

		// Aplicar sustracción de fondo
		mog2.Apply(frame, &fgMask)
		gocv.MorphologyEx(fgMask, &fgMask, gocv.MorphOpen, kernel)

		contours := gocv.FindContours(fgMask, gocv.RetrievalExternal, gocv.ChainApproxSimple)
		objectOutside := false

		for i := 0; i < contours.Size(); i++ {
			c := contours.At(i)
			area := gocv.ContourArea(c)

			if area > 1000 { // Umbral de tamaño para vehículos
				rect := gocv.BoundingRect(c)

				// Validar si el vehículo detectado es de color BLANCO por brillo promedio
				if isWhiteCar(frame, rect) {
					// Dibujar rectángulo verde brillante para autos blancos detectados
					gocv.Rectangle(&frame, rect, color.RGBA{0, 255, 0, 0}, 2)

					center := image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2)
					if !center.In(safeArea) {
						objectOutside = true
					}
				}
			}
		}
		contours.Close()

		// Dibujar la zona segura en azul
		gocv.Rectangle(&frame, safeArea, color.RGBA{255, 0, 0, 0}, 2)

		if objectOutside {
			timestamp := time.Now().Format("20060102_150405")
			filename := fmt.Sprintf("alerta_auto_blanco_%s.jpg", timestamp)
			gocv.IMWrite(filename, frame)
			fmt.Println("¡ALERTA! Auto blanco fuera del área segura. Foto guardada:", filename)
			time.Sleep(1 * time.Second)
		}

		window.IMShow(frame)
		frame.Close()

		if window.WaitKey(30) == 27 { // ESC para salir
			break
		}
	}
}

// isWhiteCar recorta el área del vehículo y evalúa si su color promedio es blanco (alta luminosidad BGR)
func isWhiteCar(frame gocv.Mat, rect image.Rect) bool {
	if rect.Min.X < 0 || rect.Min.Y < 0 || rect.Max.X > frame.Cols() || rect.Max.Y > frame.Rows() {
		return false
	}

	subMat := frame.Region(rect)
	defer subMat.Close()

	mean := gocv.Mean(subMat)
	b := mean.Val1
	g := mean.Val2
	r := mean.Val3

	// Si los canales B, G y R son altos (> 180), el objeto es predominantemente blanco o muy claro
	return b > 180 && g > 180 && r > 180
}
