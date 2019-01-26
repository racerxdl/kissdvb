#include <libswscale/swscale.h>
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavutil/frame.h>
#include <libavutil/imgutils.h>
#include <libavutil/avutil.h>

#define TRUE 1
#define FALSE 0

typedef struct {
    AVCodec *c;
    AVCodecContext *ctx;
    AVFrame *f;
    AVFrame *frgb;
    struct SwsContext * swsCtx;
    int got;
    double pts;
    double timebase;
    AVPacket packet;
    int64_t framecount;
    int64_t starttime;
    size_t packetBuffLen;
} h264dec_t ;


typedef struct {
    AVCodec *c;
    AVCodecContext *ctx;
    AVFrame *f;
    int got;

    AVPacket packet;
    size_t packetBuffLen;
    int bytesPerSample;
} aacdec_t ;


typedef struct {
    int width;
    int height;
    float frameRate;
    int ok;
    double timebase;
    int64_t starttime;
} image_params_t;

typedef struct {
    uint8_t *ptr;
    size_t size; ///< size left in the buffer
} buffer_data_t;

image_params_t getImageParams(uint8_t *buffer, int bufferSize);
int image_params_width(image_params_t *h);
int image_params_height(image_params_t *h);
int image_params_ok(image_params_t *h);
int64_t image_params_starttime(image_params_t *h) ;
double image_params_timebase(image_params_t *h);
float image_params_frameRate(image_params_t *h);
int h264dec_new(h264dec_t *h, int width, int height, double timebase, int64_t starttime);
int h264dec_width(h264dec_t *h);
int h264dec_height(h264dec_t *h);
int h264dec_sendpacket(h264dec_t *h, uint8_t *data, int len);
int h264dec_recvpacket(h264dec_t *h, uint8_t *rgbBuffer, int rgbSize);

int aacdec_new(aacdec_t *m);
int aacdec_sendpacket(aacdec_t *m, uint8_t *data, int len);
int aacdec_recvpacket(aacdec_t *m, float *audioBuffer, int audioBufferLength);
void libav_init();