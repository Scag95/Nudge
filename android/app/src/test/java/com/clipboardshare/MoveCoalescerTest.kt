package com.clipboardshare

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import kotlin.math.abs

class MoveCoalescerTest {
    private class FakeFrames : MoveCoalescer.FrameScheduler {
        val pending = mutableListOf<() -> Unit>()
        override fun postFrame(callback: () -> Unit) { pending += callback }
        fun runFrame() {
            val run = pending.toList()
            pending.clear()
            run.forEach { it() }
        }
    }

    private val log = mutableListOf<String>()
    private val frames = FakeFrames()
    private val coalescer = MoveCoalescer(frames) { dx, dy -> log += "move $dx $dy" }

    @Test fun manyMovesWithinOneFrameBecomeOneMessageWithTheSum() {
        repeat(20) { coalescer.add(1, -2) }
        assertTrue("nothing is sent before the frame", log.isEmpty())
        assertEquals("one callback per frame", 1, frames.pending.size)
        frames.runFrame()
        assertEquals(listOf("move 20 -40"), log)
        assertTrue(frames.pending.isEmpty())
    }

    @Test fun eachFrameFlushesItsOwnMoves() {
        coalescer.add(3, 0)
        frames.runFrame()
        coalescer.add(0, 4)
        coalescer.add(0, 1)
        frames.runFrame()
        assertEquals(listOf("move 3 0", "move 0 5"), log)
    }

    @Test fun anEventAfterMovesFlushesTheMovesFirst() {
        coalescer.add(5, 5)
        coalescer.add(1, 0)
        coalescer.flushThen { log += "click left tap" }
        assertEquals(listOf("move 6 5", "click left tap"), log)
        frames.runFrame() // the frame that was already scheduled finds nothing left
        assertEquals(2, log.size)
    }

    @Test fun anEventWithNothingPendingJustRuns() {
        coalescer.flushThen { log += "key a" }
        assertEquals(listOf("key a"), log)
    }

    @Test fun noMessageWhenTheSumIsZero() {
        coalescer.add(3, -2)
        coalescer.add(-3, 2)
        frames.runFrame()
        assertTrue(log.isEmpty())
        coalescer.flushThen { log += "scroll" }
        assertEquals(listOf("scroll"), log)
    }

    @Test fun aZeroMoveSchedulesNothing() {
        coalescer.add(0, 0)
        assertTrue(frames.pending.isEmpty())
    }

    @Test fun sumsOverTheWireLimitAreSplitNotDropped() {
        coalescer.add(1500, -1001)
        coalescer.add(1000, 0)
        frames.runFrame()
        val parts = log.map { l -> l.split(" ").drop(1).map { it.toInt() } }
        assertEquals(2500, parts.sumOf { it[0] })
        assertEquals(-1001, parts.sumOf { it[1] })
        assertTrue(parts.all { p -> p.all { abs(it) <= 1000 } })
        assertTrue(parts.none { it[0] == 0 && it[1] == 0 })
    }

    @Test fun cancelDropsPendingMovesAndTheScheduledFrameIsHarmless() {
        coalescer.add(9, 9)
        coalescer.cancel()
        frames.runFrame()
        assertTrue(log.isEmpty())
        coalescer.add(1, 1)
        assertEquals(1, frames.pending.size)
        frames.runFrame()
        assertEquals(listOf("move 1 1"), log)
    }
}
