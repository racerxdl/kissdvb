package main

import (
    "fmt"
    "github.com/go-gl/gl/v3.2-core/gl"
    "github.com/go-gl/glfw/v3.2/glfw"
    "github.com/golang-ui/nuklear/nk"
    "github.com/llgcode/draw2d/draw2dimg"
    "image"
    "image/color"
    "sync"
    "unsafe"
)

var drawLock = sync.Mutex{}

var constellationImage = image.NewRGBA(image.Rect(0, 0, 256, 256))
var gc = draw2dimg.NewGraphicContext(constellationImage)
var constellationFrame nk.Image
var constellationTex int32
var isUpdated bool
var monoAtlas *nk.FontAtlas
var sansAtlas *nk.FontAtlas
var fonts = make(map[string]*nk.Font)

const (
    winWidth  = 1280
    winHeight = 720

    maxVertexBuffer  = 512 * 1024
    maxElementBuffer = 128 * 1024
)

func init() {
    for i := 0; i < 128; i++ {
        for j := 0; j < 128; j++ {
            constellationImage.Set(i, j, color.Black)
        }
    }
    isUpdated = true
}

var displayData = make([]complex64, 1024)

func inBounds(v float32) float32 {
    if v > 127 {
        return 127
    } else if v < -128 {
        return -128
    }

    return v
}

func UpdateConstellation() {
    constellationSymbolFifo.UnsafeLock()
    for i := 0; i < len(displayData); i++ {
        displayData[i] = constellationSymbolFifo.UnsafeNext().(complex64)
    }
    constellationSymbolFifo.UnsafeUnlock()
    //log.Println("New Constellation!")
    drawLock.Lock()

    gc.SetFillColor(color.Black)
    gc.Clear()

    for i := 0; i < len(displayData); i++ {
        c := displayData[i]
        y := int(real(c) * 127 * 0.8) + 127
        x := int(imag(c) * 127 * 0.8) + 127

        if x < 256 && x >= 0 && y < 256 && y >= 0 {
            constellationImage.Set(x, y, color.White)
        }
    }

    isUpdated = true
    drawLock.Unlock()
}

func InitializeFonts() {
    var freeSansBytes = MustAsset("assets/FreeSans.ttf")
    var freeMonoBytes = MustAsset("assets/FreeMono.ttf")
    sansAtlas = nk.NewFontAtlas()
    nk.NkFontStashBegin(&sansAtlas)
    for i := 8; i <= 64; i += 2 {
        var fontName = fmt.Sprintf("sans%d", i)
        fonts[fontName] = nk.NkFontAtlasAddFromBytes(sansAtlas, freeSansBytes, float32(i), nil)
    }
    nk.NkFontStashEnd()
    monoAtlas = nk.NewFontAtlas()
    nk.NkFontStashBegin(&monoAtlas)
    for i := 8; i <= 64; i += 2 {
        var fontName = fmt.Sprintf("mono%d", i)
        fonts[fontName] = nk.NkFontAtlasAddFromBytes(monoAtlas, freeMonoBytes, float32(i), nil)
    }
    nk.NkFontStashEnd()
}

func rgbaTex(tex int32, rgba *image.RGBA) (nk.Image, int32) {
    var t uint32
    if tex == -1 {
        gl.GenTextures(1, &t)
    } else {
        t = uint32(tex)
    }
    gl.BindTexture(gl.TEXTURE_2D, t)
    gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_NEAREST)
    gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR_MIPMAP_NEAREST)
    gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
    gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
    gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(rgba.Bounds().Dx()), int32(rgba.Bounds().Dy()),
        0, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&rgba.Pix[0]))
    gl.GenerateMipmap(gl.TEXTURE_2D)
    return nk.NkImageId(int32(t)), int32(t)
}

func DrawConstellation(win *glfw.Window, ctx *nk.Context) {
    width, height := win.GetSize()
    bounds := nk.NkRect(0, 0, float32(width), float32(height))
    update := nk.NkBegin(ctx, "Window", bounds, 0)
    if update > 0 {
        if isUpdated {
            constellationFrame, constellationTex = rgbaTex(constellationTex, constellationImage)
            isUpdated = false
        }
        nk.NkLayoutRowStatic(ctx, 256, 256, 1)
        {
            nk.NkImage(ctx, constellationFrame)
        }
    }
    nk.NkEnd(ctx)
}

func gfxMain(win *glfw.Window, ctx *nk.Context) {
    drawLock.Lock()
    defer drawLock.Unlock()
    width, height := win.GetSize()
    nk.NkPlatformNewFrame()

    DrawConstellation(win, ctx)

    // Render
    // 28, 48, 62, 255
    bg := make([]float32, 4)
    bg[0] = 28
    bg[1] = 48
    bg[2] = 62
    bg[3] = 255
    gl.Viewport(0, 0, int32(width), int32(height))
    gl.Clear(gl.COLOR_BUFFER_BIT)
    gl.ClearColor(bg[0], bg[1], bg[2], bg[3])
    nk.NkPlatformRender(nk.AntiAliasingOn, maxVertexBuffer, maxElementBuffer)
    win.SwapBuffers()
}