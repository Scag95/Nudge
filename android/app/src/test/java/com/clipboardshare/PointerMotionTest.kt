package com.clipboardshare

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class PointerMotionTest {
    private fun linear(speed: Double = 1.0) =
        PointerMotion(speed, PointerMotion.Acceleration.OFF)

    /** Feeds [n] equal moves 16 ms apart; returns the emitted steps (null = nothing sent). */
    private fun feed(m: PointerMotion, dx: Float, dy: Float, n: Int, dtMs: Long = 16L): List<PointerMotion.Step?> =
        (0 until n).map { m.map(dx, dy, it * dtMs) }

    @Test fun tenSubPixelMovesAccumulateToThreePixelsWithNoZeroFrames() {
        val steps = feed(PointerMotion(), 0.3f, 0f, 10)
        val sent = steps.filterNotNull()
        assertEquals(3, sent.sumOf { it.dx })
        assertEquals(0, sent.sumOf { it.dy })
        assertTrue(sent.none { it.dx == 0 && it.dy == 0 })
    }

    @Test fun neverEmitsAZeroStep() {
        val m = linear()
        assertNull(m.map(0.4f, 0.4f, 0))
        assertNull(m.map(0f, 0f, 16))
        assertNotNull(m.map(0.7f, 0f, 32)) // 1.1 on x, 0.4 on y carried
    }

    @Test fun negativeFractionsTruncateTowardZeroAndKeepTheSignedRemainder() {
        val m = linear()
        assertEquals(PointerMotion.Step(-1, 0), m.map(-1.5f, 0f, 0))
        // remainder is -0.5: another -0.5 completes a pixel
        assertEquals(PointerMotion.Step(-1, 0), m.map(-0.5f, 0f, 16))
        val total = feed(PointerMotion(), -0.3f, 0.3f, 10).filterNotNull()
        assertEquals(-3, total.sumOf { it.dx })
        assertEquals(3, total.sumOf { it.dy })
    }

    @Test fun axesAccumulateIndependently() {
        val m = linear()
        assertEquals(PointerMotion.Step(1, 0), m.map(1.2f, 0.4f, 0))
        assertEquals(PointerMotion.Step(0, 1), m.map(0.1f, 0.7f, 16))
    }

    @Test fun resetClearsTheRemainder() {
        val m = linear()
        assertNull(m.map(0.6f, 0f, 0))
        m.reset()
        assertNull(m.map(0.6f, 0f, 0)) // would be 1 px with a stale 0.6 remainder
    }

    @Test fun speedScalesTheMotion() {
        assertEquals(PointerMotion.Step(3, -6), linear(2.0).map(1.5f, -3f, 0))
        val half = linear(0.5)
        assertNull(half.map(1f, 0f, 0))
        assertEquals(PointerMotion.Step(1, 0), half.map(1f, 0f, 16))
    }

    @Test fun speedIsClampedToItsRange() {
        val m = PointerMotion()
        m.speed = 10.0
        assertEquals(PointerMotion.MAX_SPEED, m.speed, 0.0)
        m.speed = 0.0
        assertEquals(PointerMotion.MIN_SPEED, m.speed, 0.0)
        assertEquals(1.0, PointerMotion.DEFAULT_SPEED, 0.0)
        assertEquals(PointerMotion.Acceleration.SUAVE, PointerMotion().acceleration)
    }

    @Test fun accelerationOffIsLinearAtAnyVelocity() {
        val off = PointerMotion.Acceleration.OFF
        for (v in listOf(0.0, 0.1, 1.0, 5.0, 50.0)) assertEquals(1.0, off.gain(v), 0.0)
        val m = linear()
        // 40 px in 2 ms is very fast, yet exactly 1:1
        assertEquals(PointerMotion.Step(40, 0), m.map(40f, 0f, 0).also { })
        assertEquals(PointerMotion.Step(40, 0), m.map(40f, 0f, 2))
    }

    @Test fun slowPreciseMovesAreOneToOneAtTheDefault() {
        for (a in PointerMotion.Acceleration.values()) {
            assertEquals(1.0, a.gain(0.0), 0.0)
            assertEquals(1.0, a.gain(0.1), 0.0)
        }
        val m = PointerMotion()
        // 1 px every 16 ms = 0.06 px/ms
        val sent = feed(m, 1f, 0f, 20).filterNotNull()
        assertEquals(20, sent.sumOf { it.dx })
    }

    @Test fun accelerationIsMonotonicAndCapped() {
        for (a in listOf(PointerMotion.Acceleration.SUAVE, PointerMotion.Acceleration.ALTA)) {
            var prev = a.gain(0.0)
            var v = 0.0
            while (v <= 30.0) {
                val g = a.gain(v)
                assertTrue("$a gain($v)=$g < $prev", g >= prev)
                assertTrue("$a gain($v)=$g above cap", g <= a.maxGain)
                assertTrue(g >= 1.0)
                prev = g
                v += 0.05
            }
            assertEquals(a.maxGain, a.gain(30.0), 0.0)
            assertTrue(a.maxGain > 1.0)
        }
        assertTrue(
            PointerMotion.Acceleration.ALTA.maxGain > PointerMotion.Acceleration.SUAVE.maxGain
        )
        for (v in listOf(0.5, 1.0, 2.0, 4.0)) {
            assertTrue(
                PointerMotion.Acceleration.ALTA.gain(v) >= PointerMotion.Acceleration.SUAVE.gain(v)
            )
        }
    }

    @Test fun fastMovesAreAmplifiedUpToTheCap() {
        val m = PointerMotion(1.0, PointerMotion.Acceleration.SUAVE)
        // 10 px every 2 ms = 5 px/ms, far past the knee
        val last = feed(m, 10f, 0f, 8, dtMs = 2).last()!!
        assertEquals((10 * PointerMotion.Acceleration.SUAVE.maxGain).toInt(), last.dx)
    }

    @Test fun accelerationCombinesWithSpeed() {
        val m = PointerMotion(2.0, PointerMotion.Acceleration.SUAVE)
        // slow: gain 1, so exactly speed
        assertEquals(PointerMotion.Step(2, 0), m.map(1f, 0f, 0))
    }

    @Test fun accelerationNamesRoundTripAndFallBackToTheDefault() {
        for (a in PointerMotion.Acceleration.values()) {
            assertEquals(a, PointerMotion.Acceleration.fromName(a.name))
        }
        assertEquals(PointerMotion.Acceleration.SUAVE, PointerMotion.Acceleration.fromName(null))
        assertEquals(PointerMotion.Acceleration.SUAVE, PointerMotion.Acceleration.fromName("???"))
    }

    @Test fun sliderProgressMapsToSpeedAndBack() {
        assertEquals(PointerMotion.MIN_SPEED, PointerMotion.speedFromProgress(0), 1e-9)
        assertEquals(PointerMotion.MAX_SPEED, PointerMotion.speedFromProgress(PointerMotion.SPEED_STEPS), 1e-9)
        assertEquals(PointerMotion.MAX_SPEED, PointerMotion.speedFromProgress(999), 1e-9)
        for (p in 0..PointerMotion.SPEED_STEPS) {
            assertEquals(p, PointerMotion.progressFromSpeed(PointerMotion.speedFromProgress(p)))
        }
        assertEquals(1.0, PointerMotion.speedFromProgress(PointerMotion.progressFromSpeed(1.0)), 1e-9)
    }
}
