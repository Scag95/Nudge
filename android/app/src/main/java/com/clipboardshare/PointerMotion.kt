package com.clipboardshare

import kotlin.math.abs
import kotlin.math.hypot
import kotlin.math.max

/**
 * Turns raw finger deltas (float pixels) into the whole-pixel `move dx dy` the
 * protocol carries, without losing motion on the way.
 *
 * Pipeline per event:
 *  1. velocity v = |delta| / dt in px/ms, lightly smoothed (EMA) so the gain does not
 *     jitter with the 1-px quantisation of touch coordinates;
 *  2. gain  g = [Acceleration.gain] (v)  — 1.0 for slow moves, growing smoothly with v;
 *  3. scaled = delta * [speed] * g;
 *  4. the per-axis fractional remainder is carried: the emitted integer is
 *     `trunc(remainder + scaled)` (toward zero, so negatives behave symmetrically) and
 *     the signed rest stays for the next event. An event that yields (0, 0) returns null.
 *
 * Acceleration curve (smoothstep between a knee and a saturation velocity):
 *
 *     t    = clamp((v - kneeVelocity) / (maxVelocity - kneeVelocity), 0, 1)
 *     gain = 1 + (maxGain - 1) * t * t * (3 - 2 * t)
 *
 * so below the knee the pointer is exactly 1:1 (precise moves feel unchanged), it eases
 * in without a kink, is monotonic in v and never exceeds maxGain:
 *
 *     SUAVE (default): knee 0.4 px/ms, saturates at 3.0 px/ms, max gain 2.0x
 *     ALTA:            knee 0.25 px/ms, saturates at 2.5 px/ms, max gain 3.5x
 *     OFF:             gain 1.0 always (linear)
 *
 * Pure Kotlin on purpose: no Android types, unit-tested on the JVM. Not thread-safe;
 * call it from the UI thread only.
 */
class PointerMotion(
    speed: Double = DEFAULT_SPEED,
    var acceleration: Acceleration = Acceleration.SUAVE,
) {
    enum class Acceleration(
        val label: String,
        private val kneeVelocity: Double,
        private val maxVelocity: Double,
        val maxGain: Double,
    ) {
        OFF("Off", 0.0, 1.0, 1.0),
        SUAVE("Suave", 0.4, 3.0, 2.0),
        ALTA("Alta", 0.25, 2.5, 3.5);

        /** Multiplier for a finger moving at [velocity] px/ms; 1.0..[maxGain], monotonic. */
        fun gain(velocity: Double): Double {
            if (maxGain <= 1.0) return 1.0
            val t = ((velocity - kneeVelocity) / (maxVelocity - kneeVelocity)).coerceIn(0.0, 1.0)
            return minOf(maxGain, 1.0 + (maxGain - 1.0) * t * t * (3.0 - 2.0 * t))
        }

        companion object {
            val DEFAULT = SUAVE

            fun fromName(name: String?): Acceleration =
                values().firstOrNull { it.name == name } ?: DEFAULT
        }
    }

    /** Whole pixels to send; never (0, 0). */
    data class Step(val dx: Int, val dy: Int)

    /** Cursor speed multiplier, always within [MIN_SPEED]..[MAX_SPEED]. */
    var speed: Double = speed.coerceIn(MIN_SPEED, MAX_SPEED)
        set(value) { field = value.coerceIn(MIN_SPEED, MAX_SPEED) }

    private var remX = 0.0
    private var remY = 0.0
    private var lastTimeMs = -1L
    private var smoothedVelocity = 0.0

    /** Call on ACTION_DOWN/UP/CANCEL (and any time the finger identity changes). */
    fun reset() {
        remX = 0.0
        remY = 0.0
        lastTimeMs = -1L
        smoothedVelocity = 0.0
    }

    /** Maps one raw delta observed at [timeMs]; null when no whole pixel is due yet. */
    fun map(dx: Float, dy: Float, timeMs: Long): Step? {
        if (lastTimeMs >= 0) {
            val dt = max(timeMs - lastTimeMs, 1L)
            val v = hypot(dx.toDouble(), dy.toDouble()) / dt
            smoothedVelocity = VELOCITY_SMOOTHING * v + (1 - VELOCITY_SMOOTHING) * smoothedVelocity
        }
        lastTimeMs = timeMs
        val k = speed * acceleration.gain(smoothedVelocity)
        val ax = remX + dx * k
        val ay = remY + dy * k
        val ix = whole(ax)
        val iy = whole(ay)
        remX = ax - ix
        remY = ay - iy
        return if (ix == 0 && iy == 0) null else Step(ix, iy)
    }

    /** Truncates toward zero; EPSILON absorbs float error so ten 0.3 px moves are 3 px. */
    private fun whole(v: Double): Int = (if (v >= 0) v + EPSILON else v - EPSILON).toInt()

    companion object {
        const val MIN_SPEED = 0.5
        const val MAX_SPEED = 3.0
        const val DEFAULT_SPEED = 1.0

        /** The speed slider has SPEED_STEPS + 1 positions of 0.1x each (0.5x .. 3.0x). */
        const val SPEED_STEPS = 25
        private const val SPEED_INCREMENT = 0.1
        private const val VELOCITY_SMOOTHING = 0.5
        private const val EPSILON = 1e-6

        fun speedFromProgress(progress: Int): Double =
            (MIN_SPEED + progress.coerceIn(0, SPEED_STEPS) * SPEED_INCREMENT).coerceIn(MIN_SPEED, MAX_SPEED)

        fun progressFromSpeed(speed: Double): Int =
            Math.round((speed.coerceIn(MIN_SPEED, MAX_SPEED) - MIN_SPEED) / SPEED_INCREMENT).toInt()
    }
}
