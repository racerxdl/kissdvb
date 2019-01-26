#include "h264.h"

AVFrame * icv_alloc_picture_FFMPEG(int pix_fmt, int width, int height, int alloc) {
    // printf("Alocating frame %dx%d pixels\n", width, height);
    AVFrame * picture;
    uint8_t * picture_buf;
    int size;

    picture = av_frame_alloc();
    if (!picture)
        return NULL;
    size = av_image_get_buffer_size(pix_fmt, width, height, 1);
    if(alloc)
    {
        picture_buf = (uint8_t *) malloc(size);
        if (!picture_buf)
        {
            av_frame_free (&picture);
            return NULL;
        }
        av_image_fill_arrays(picture->data, picture->linesize, picture_buf, pix_fmt, width, height, 1);
    }
    picture->pts = 0;
    return picture;
}

int read_packet(void *opaque, uint8_t *buf, int buf_size) {
    buffer_data_t *bd = (buffer_data_t *)opaque;
    buf_size = FFMIN(buf_size, bd->size);
    // copy internal buffer data to buf
    memcpy(buf, bd->ptr, buf_size);
    bd->ptr  += buf_size;
    bd->size -= buf_size;
    return buf_size;
}

image_params_t getImageParams(uint8_t *buffer, int bufferSize) {
  image_params_t params;
  params.ok = FALSE;
  // Format Detection
  AVFormatContext *fmt_ctx = NULL;
  AVIOContext *avio_ctx = NULL;
  uint8_t *avio_ctx_buffer = NULL;
  size_t avio_ctx_buffer_size = 4096;
  buffer_data_t bd;
  int ret = 0;
  bd.ptr  = buffer;
  bd.size = bufferSize;
  if (!(fmt_ctx = avformat_alloc_context())) {
      ret = AVERROR(ENOMEM);
      return params;
  }
  avio_ctx_buffer = (uint8_t *)av_malloc(avio_ctx_buffer_size);
  if (!avio_ctx_buffer) {
      ret = AVERROR(ENOMEM);
      return params;
  }
  avio_ctx = avio_alloc_context(avio_ctx_buffer, avio_ctx_buffer_size,
                                0, &bd, &read_packet, NULL, NULL);
  if (!avio_ctx) {
      ret = AVERROR(ENOMEM);
      return params;
  }
  fmt_ctx->pb = avio_ctx;
  ret = avformat_open_input(&fmt_ctx, NULL, NULL, NULL);
  if (ret < 0) {
      fprintf(stderr, "Could not open input\n");
      return params;
  }
  ret = avformat_find_stream_info(fmt_ctx, NULL);
  if (ret < 0) {
      fprintf(stderr, "Could not find stream information\n");
      return params;
  }

  params.width = fmt_ctx->streams[0]->codecpar->width;
  params.height = fmt_ctx->streams[0]->codecpar->height;
  params.frameRate = (float)fmt_ctx->streams[0]->r_frame_rate.num / (float)fmt_ctx->streams[0]->r_frame_rate.den;
  params.timebase = (float)fmt_ctx->streams[0]->r_frame_rate.den / (float)fmt_ctx->streams[0]->r_frame_rate.num;
  params.starttime = fmt_ctx->streams[0]->start_time;
  if (params.starttime == AV_NOPTS_VALUE) {
    params.starttime = 0;
  }
  params.ok = !isnan(params.frameRate);
  return params;
}

int image_params_width(image_params_t *h) {
    return h->width;
}
int image_params_height(image_params_t *h) {
    return h->height;
}
int image_params_ok(image_params_t *h) {
    return h->ok;
}
int64_t image_params_starttime(image_params_t *h) {
    return h->starttime;
}
float image_params_frameRate(image_params_t *h) {
    return h->frameRate;
}
double image_params_timebase(image_params_t *h) {
    return h->timebase;
}
int h264dec_new(h264dec_t *h, int width, int height, double timebase, int64_t starttime) {
    h->c = avcodec_find_decoder(AV_CODEC_ID_H264);
    h->ctx = avcodec_alloc_context3(h->c);
    h->f = icv_alloc_picture_FFMPEG(AV_PIX_FMT_YUV420P, width, height, TRUE);
    h->frgb = icv_alloc_picture_FFMPEG(AV_PIX_FMT_RGBA, width, height, TRUE);
    h->ctx->extradata = NULL;
    h->ctx->debug = 0x3;
    h->ctx->width = width;
    h->ctx->height = height;
    h->ctx->pix_fmt = AV_PIX_FMT_YUV420P;
    h->swsCtx = sws_getContext(width, height, AV_PIX_FMT_YUV420P, width, height, AV_PIX_FMT_RGBA, SWS_FAST_BILINEAR, NULL, NULL, NULL);
    h->timebase = timebase;
    h->framecount = 0;
    h->starttime = starttime;
    h->f->pts = starttime;
    h->frgb->pts = starttime;

    av_init_packet(&h->packet);
    h->packetBuffLen = 1024 * 1024; // 1MB
    h->packet.size = h->packetBuffLen;
    h->packet.data = malloc(h->packetBuffLen);
    return avcodec_open2(h->ctx, h->c, NULL);
}

int h264dec_width(h264dec_t *h) {
    return h->ctx->width;
}

int h264dec_height(h264dec_t *h) {
    return h->ctx->height;
}

int h264dec_sendpacket(h264dec_t *h, uint8_t *data, int len) {
    if (h->packetBuffLen < len) {
        free(h->packet.data);
        h->packetBuffLen = len;
        h->packet.data = malloc(h->packetBuffLen);
        h->packet.size = h->packetBuffLen;
    }
    h->packet.size = len;

    memcpy(h->packet.data, data, len);

    return avcodec_send_packet(h->ctx, &h->packet);
}

int h264dec_recvpacket(h264dec_t *h, uint8_t *rgbBuffer, int rgbSize) {
    int av_return = avcodec_receive_frame(h->ctx, h->f);
    if (av_return >= 0) {
        h->got = TRUE;
        sws_scale(h->swsCtx, (const uint8_t * const*)h->f->data, h->f->linesize, 0, h->ctx->height, h->frgb->data, h->frgb->linesize);
        av_image_copy_to_buffer((unsigned char *)rgbBuffer, rgbSize, (const uint8_t **)h->frgb->data, h->frgb->linesize, AV_PIX_FMT_RGBA, h->ctx->width, h->ctx->height, 1);

        if (h->packet.dts != AV_NOPTS_VALUE) {
            h->pts = av_frame_get_best_effort_timestamp(h->f);
        } else {
            h->pts = 0;
        }

        h->pts = h->framecount * h->timebase;
        h->framecount++;
    }

    return av_return;
}

int aacdec_new(aacdec_t *m) {
    m->c = avcodec_find_decoder(AV_CODEC_ID_AAC);
    m->ctx = avcodec_alloc_context3(m->c);
    m->f = av_frame_alloc();
    m->ctx->extradata = NULL;
    m->ctx->debug = 0x3;
    m->bytesPerSample = 0;

    av_init_packet(&m->packet);
    m->packetBuffLen = 1024 * 1024; // 1MB
    m->packet.size = m->packetBuffLen;
    m->packet.data = malloc(m->packetBuffLen);

    return avcodec_open2(m->ctx, m->c, 0);
}

int aacdec_sendpacket(aacdec_t *m, uint8_t *data, int len) {
    if (m->packetBuffLen < len) {
        free(m->packet.data);
        m->packetBuffLen = len;
        m->packet.data = malloc(m->packetBuffLen);
        m->packet.size = m->packetBuffLen;
    }
    m->packet.size = len;

    memcpy(m->packet.data, data, len);
    return avcodec_send_packet(m->ctx, &m->packet);
}


int aacdec_recvpacket(aacdec_t *m, float *audioBuffer, int audioBufferLength) {
    int ret = avcodec_receive_frame(m->ctx, m->f);

    if (ret >= 0) {
        m->bytesPerSample = av_get_bytes_per_sample(m->f->format);
        m->got = 1;
        int bytesToCopy = m->f->linesize[0] / m->f->channels;
        if (audioBufferLength < bytesToCopy) {
            bytesToCopy = audioBufferLength;
        }
        memcpy(audioBuffer, m->f->data[0], bytesToCopy);
    }

    return ret;
}

void libav_init() {
    av_register_all();
    avcodec_register_all();
    // av_log_set_level(AV_LOG_DEBUG);
}