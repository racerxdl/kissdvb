package main

import (
    "github.com/OpenSatelliteProject/libsathelper"
    "github.com/go-gl/gl/v3.2-core/gl"
    "github.com/go-gl/glfw/v3.2/glfw"
    "github.com/golang-ui/nuklear/nk"
    "github.com/racerxdl/go.fifo"
    "github.com/racerxdl/kissdvb/Frontend"
    "github.com/racerxdl/kissdvb/Frontend/CFileFrontend"
    "github.com/racerxdl/segdsp/dsp"
    "log"
    "os"
    "os/signal"
    "runtime"
    "sync"
    "syscall"
    "time"
)


const PllAlpha float32 = 0.0001
const ClockAlpha float32 = 0.0037
const ClockMu float32 = 0.5
const ClockOmegaLimit float32 = 0.005
const ClockGainOmega = (ClockAlpha * ClockAlpha) / 4.0


var filter *dsp.FirFilter
var costasNew dsp.CostasLoop

var buffer0 []complex64
var buffer1 []complex64

var mmOld SatHelper.ClockRecovery

func checkAndResizeBuffers(length int) {
    if len(buffer0) < length {
        buffer0 = make([]complex64, length)
    }
    if len(buffer1) < length {
        buffer1 = make([]complex64, length)
    }
}

func swapBuffers(a *[]complex64, b *[]complex64) {
    c := *b
    *b = *a
    *a = c
}

var lock = sync.Mutex{}
var lastConstellationUpdate time.Time

var constellationSymbolFifo *fifo.Queue

func sampleLoop(data []complex64) {
    lock.Lock()
    defer lock.Unlock()

    checkAndResizeBuffers(len(data))

    copy(buffer0, data)

    ba := buffer0
    bb := buffer1

    s := filter.WorkBuffer(ba, bb)
    swapBuffers(&ba, &bb)

    s = costasNew.WorkBuffer(ba, bb)
    swapBuffers(&ba, &bb)

    //s = mmNew.WorkBuffer(ba, bb)
    s = mmOld.Work(&ba[0], &bb[0], s)
    swapBuffers(&ba, &bb)

    constellationSymbolFifo.UnsafeLock()
    for i := 0; i < s; i++ {
        if constellationSymbolFifo.UnsafeLen() > 2048 {
            break
        }
        constellationSymbolFifo.UnsafeAdd(ba[i])
    }
    constellationSymbolFifo.UnsafeUnlock()

    if time.Since(lastConstellationUpdate) > time.Millisecond * 10 && constellationSymbolFifo.Len() >= 1024 {
        go UpdateConstellation()
        lastConstellationUpdate = time.Now()
    }

    DecodePut(ba[:s])
}


func main() {
    frontend := CFileFrontend.NewCFileFrontend("/media/ELTN/Baseband Records/DVB-S/dvbs-2e6.cfile")
    frontend.SetSampleRate(2e6)

    sampleRate := 2e6
    symbolRate := 1e6
    sps := sampleRate / symbolRate

    rrcTaps := dsp.MakeRRC(1, sampleRate, symbolRate, 0.35, 31)
    filter = dsp.MakeFirFilter(rrcTaps)
    costasNew = dsp.MakeCostasLoop4(PllAlpha)

    //mmOld = SatHelper.NewClockRecovery(float32(sps), 0.25*0.175*0.175, 0.5, 0.175, 0.005)
    mmOld = SatHelper.NewClockRecovery(float32(sps), ClockGainOmega, ClockMu, ClockAlpha, ClockOmegaLimit)
    //mmNew = digital.NewComplexClockRecovery(float32(sps), 0.25*0.175*0.175, 0.5, 0.175, 0.005)
    lastConstellationUpdate = time.Now()
    constellationSymbolFifo = fifo.NewQueue()

    frontend.SetSamplesAvailableCallback(func(data Frontend.SampleCallbackData) {
        sampleLoop(data.ComplexArray)
    })

    runtime.LockOSThread()
    if err := glfw.Init(); err != nil {
        log.Fatalln(err)
    }
    glfw.WindowHint(glfw.ContextVersionMajor, 3)
    glfw.WindowHint(glfw.ContextVersionMinor, 2)
    glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
    glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
    win, err := glfw.CreateWindow(winWidth, winHeight, "KissDVB", nil, nil)
    if err != nil {
        log.Fatalln(err)
    }
    win.MakeContextCurrent()

    width, height := win.GetSize()

    if err := gl.Init(); err != nil {
        log.Fatalf("opengl initialization failed: %s", err)
    }
    gl.Viewport(0, 0, int32(width), int32(height))

    ctx := nk.NkPlatformInit(win, nk.PlatformInstallCallbacks)

    exitC := make(chan os.Signal, 1)
    doneC := make(chan struct{}, 1)

    of, _ = os.Create("test.bin")

    signal.Notify(exitC, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-exitC
        log.Println("Got SIGTERM!")
        _ = of.Close()
        frontend.Stop()
        win.SetShouldClose(true)
        <-doneC
    }()

    fpsTicker := time.NewTicker(time.Second / 60)

    constellationFrame, constellationTex = rgbaTex(constellationTex, constellationImage)
    InitializeFonts()

    nk.NkStyleSetFont(ctx, fonts["sans16"].Handle())

    frontend.Start()

    for {
        select {
        case <-exitC:
            nk.NkPlatformShutdown()
            glfw.Terminate()
            fpsTicker.Stop()
            close(doneC)
            return
        case <-fpsTicker.C:
            if win.ShouldClose() {
                close(exitC)
                continue
            }
            glfw.PollEvents()
            gfxMain(win, ctx)
        }
    }
}
