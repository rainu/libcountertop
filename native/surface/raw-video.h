// SPDX-License-Identifier: AGPL-3.0-or-later
// Software-decoded RGBA follows exactly the hardware video's media clock,
// placement, pause, hide, flush and lifetime. Four frames maximum in native RAM.
static void raw_clear(video *v) {
    pthread_mutex_lock(&v->mu);
    for (int i = 0; i < 3; i++) {
	free(v->raw[i].data);
	v->raw[i].data = NULL;
    }
    v->raw_head = v->raw_count = 0;
    pthread_mutex_unlock(&v->mu);
}
static void *raw_output(void *arg) {
    video *v = arg;
    int64_t rendered = 0;
    while (!v->closing) {
	if (v->flushing) {
	    park(v);
	    continue;
	}
	pthread_mutex_lock(&v->mu);
	if (!v->raw_count) {
	    pthread_mutex_unlock(&v->mu);
	    usleep(2000);
	    continue;
	}
	raw_frame f = v->raw[v->raw_head];
	v->raw[v->raw_head].data = NULL;
	v->raw_head = (v->raw_head + 1) % 3;
	v->raw_count--;
	pthread_mutex_unlock(&v->mu);
	if (f.flags & SAMPLE_END) {
	    v->ended = 1;
	    free(f.data);
	    continue;
	}
	int render = 1, abandon = 0;
	for (;;) {
	    if (v->closing || v->flushing) {
		abandon = 1;
		break;
	    }
	    int running;
	    int64_t now = media_now(v, &running);
	    if (f.pts < now - LATE_US) {
		render = mono_us() - rendered >= STALE_US;
		break;
	    }
	    if (!v->shown || (running && f.pts <= now + EARLY_US))
		break;
	    usleep(2000);
	}
	if (abandon) {
	    free(f.data);
	    continue;
	}
	if (render) {
	    ANativeWindow_Buffer b;
	    if (ANativeWindow_lock(v->win, &b, NULL)) {
		free(f.data);
		v->failed = 1;
		return NULL;
	    }
	    if (b.width < (int)v->w || b.height < (int)v->h ||
		b.format != WINDOW_FORMAT_RGBA_8888) {
		ANativeWindow_unlockAndPost(v->win);
		free(f.data);
		v->failed = 1;
		return NULL;
	    }
	    for (unsigned y = 0; y < v->h; y++)
		memcpy((uint8_t *)b.bits + y * b.stride * 4, f.data + y * v->w * 4, v->w * 4);
	    if (ANativeWindow_unlockAndPost(v->win)) {
		free(f.data);
		v->failed = 1;
		return NULL;
	    }
	    v->shown++;
	    rendered = mono_us();
	} else
	    v->dropped++;
	free(f.data);
    }
    return NULL;
}
static int raw_sample(video *v, int64_t pts, uint32_t flags, const drm_sample *crypt,
		      const uint8_t *data, uint32_t len) {
    if ((crypt && crypt->mode != CRYPT_CLEAR) || (flags & ~(SAMPLE_KEY | SAMPLE_HIDE | SAMPLE_END)))
	return ERR_ARGS;
    if (flags & SAMPLE_END) {
	if (len)
	    return ERR_ARGS;
    } else if (len != v->w * v->h * 4)
	return ERR_ARGS;
    if (flags & SAMPLE_HIDE)
	return OK;
    pthread_mutex_lock(&v->mu);
    if (v->raw_count == 3) {
	pthread_mutex_unlock(&v->mu);
	return ERR_AGAIN;
    }
    uint8_t *copy = NULL;
    if (len) {
	copy = malloc(len);
	if (!copy) {
	    pthread_mutex_unlock(&v->mu);
	    return ERR_MEMORY;
	}
	memcpy(copy, data, len);
    }
    int slot = (v->raw_head + v->raw_count) % 3;
    v->raw[slot] = (raw_frame){copy, pts, flags};
    v->raw_count++;
    pthread_mutex_unlock(&v->mu);
    return OK;
}
