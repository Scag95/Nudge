package com.clipboardshare

import android.view.Choreographer

/**
 * Batches pointer moves so at most one `move` goes out per display frame, carrying the
 * sum of everything mapped since the last one. Nothing is dropped: the sum is exact,
 * and any other event (click, scroll, key, text) goes through [flushThen] so pending
 * motion is sent first and ordering is preserved. Sums beyond the wire limit
 * ([MAX_DELTA] per axis) are split over several messages. UI-thread only.
 */
class MoveCoalescer(
    private val frames: FrameScheduler,
    private val sink: (dx: Int, dy: Int) -> Unit,
) {
    /** Runs [postFrame]'s callback once, on the next display frame. */
    fun interface FrameScheduler {
        fun postFrame(callback: () -> Unit)
    }

    private var pendingX = 0
    private var pendingY = 0
    private var scheduled = false

    fun add(dx: Int, dy: Int) {
        if (dx == 0 && dy == 0) return
        pendingX += dx
        pendingY += dy
        if (!scheduled) {
            scheduled = true
            frames.postFrame {
                scheduled = false
                flush()
            }
        }
    }

    /** Sends what is pending now (nothing if the sum is (0, 0)). */
    fun flush() {
        var x = pendingX
        var y = pendingY
        pendingX = 0
        pendingY = 0
        while (x != 0 || y != 0) {
            val sx = x.coerceIn(-MAX_DELTA, MAX_DELTA)
            val sy = y.coerceIn(-MAX_DELTA, MAX_DELTA)
            sink(sx, sy)
            x -= sx
            y -= sy
        }
    }

    /** Sends pending motion, then runs [action] (the click/scroll/key being sent). */
    fun flushThen(action: () -> Unit) {
        flush()
        action()
    }

    /** Forgets pending motion (connection gone, screen closing). */
    fun cancel() {
        pendingX = 0
        pendingY = 0
    }

    companion object {
        /** The server rejects |dx| or |dy| above 1000. */
        const val MAX_DELTA = 1000
    }
}

/** Frame scheduler backed by the display's vsync. */
class ChoreographerFrames : MoveCoalescer.FrameScheduler {
    override fun postFrame(callback: () -> Unit) {
        Choreographer.getInstance().postFrameCallback { callback() }
    }
}
