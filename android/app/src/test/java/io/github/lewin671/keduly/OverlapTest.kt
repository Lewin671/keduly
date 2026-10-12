package io.github.lewin671.keduly

import io.github.lewin671.keduly.core.Placed
import io.github.lewin671.keduly.core.layoutDay
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class OverlapTest {
    private data class Block(val name: String, val s: Int, val e: Int)

    private fun layout(vararg blocks: Block, min: Int = 25): Map<String, Placed<Block>> =
        layoutDay(blocks.toList(), min, { it.s }, { it.e }).associateBy { it.entry?.name ?: "+" }

    private fun at(h: Int, m: Int = 0) = h * 60 + m

    @Test fun aLoneBlockTakesTheWholeColumn() {
        val a = layout(Block("a", at(9), at(10)))["a"]!!
        assertEquals(0 to 1, a.lane to a.lanes)
    }

    @Test fun backToBackBlocksShareOneLane() {
        val out = layout(Block("a", at(9), at(10)), Block("b", at(10), at(11)))
        assertEquals(0 to 1, out["a"]!!.lane to out["a"]!!.lanes)
        assertEquals(0 to 1, out["b"]!!.lane to out["b"]!!.lanes)
    }

    @Test fun twoOverlappingBlocksSitSideBySide() {
        val out = layout(Block("b", at(10, 30), at(11, 30)), Block("a", at(10), at(11, 30)))
        assertEquals(0 to 2, out["a"]!!.lane to out["a"]!!.lanes)
        assertEquals(1 to 2, out["b"]!!.lane to out["b"]!!.lanes)
    }

    @Test fun theLongerOfTwoBlocksWithTheSameStartComesFirst() {
        val out = layout(Block("short", at(10), at(10, 30)), Block("long", at(10), at(12)))
        assertEquals(0, out["long"]!!.lane)
        assertEquals(1, out["short"]!!.lane)
    }

    @Test fun aLaneIsReusedOnceItsBlockHasEnded() {
        val out = layout(Block("long", at(9), at(12)), Block("first", at(9), at(10)), Block("second", at(10), at(11)))
        assertEquals(listOf(0, 1, 1), listOf("long", "first", "second").map { out[it]!!.lane })
        assertTrue(out.values.all { it.lanes == 2 })
    }

    @Test fun aGapStartsANewCluster() {
        val out = layout(Block("a", at(9), at(10)), Block("b", at(9, 30), at(10, 30)), Block("later", at(14), at(15)))
        assertEquals(2, out["a"]!!.lanes)
        assertEquals(0 to 1, out["later"]!!.lane to out["later"]!!.lanes)
    }

    @Test fun threeOverlappingBlocksUseThreeLanes() {
        val out = layout(Block("a", at(10), at(11)), Block("b", at(10, 10), at(11)), Block("c", at(10, 20), at(11)))
        assertEquals(listOf(0, 1, 2), listOf("a", "b", "c").map { out[it]!!.lane })
        assertTrue(out.values.all { it.lanes == 3 && it.extra.isEmpty() })
    }

    @Test fun fromTheFourthBlockTheThirdLaneFoldsIntoOne() {
        // The crowded morning of the mockup.
        val out = layout(
            Block("check", at(10), at(11, 30)), Block("talk", at(10), at(11)), Block("reading", at(10, 15), at(11)),
            Block("sync", at(10, 30), at(10, 45)), Block("call", at(10, 40), at(11, 40)),
        )
        assertEquals(setOf("check", "talk", "+"), out.keys)
        assertEquals(0 to 3, out["check"]!!.lane to out["check"]!!.lanes)
        assertEquals(1 to 3, out["talk"]!!.lane to out["talk"]!!.lanes)
        val plus = out["+"]!!
        assertEquals(2 to 3, plus.lane to plus.lanes)
        assertEquals(listOf("reading", "sync", "call"), plus.extra.map { it.name })
        assertEquals(at(10, 15) to at(11, 40), plus.start to plus.end)
    }

    @Test fun aShortBlockCollidesForAsLongAsItIsDrawn() {
        // 15 minutes are drawn 25 minutes tall, so a block starting right after still collides.
        val out = layout(Block("pill", at(13), at(13, 15)), Block("next", at(13, 15), at(14)))
        assertEquals(at(13, 25), out["pill"]!!.end)
        assertEquals(0 to 2, out["pill"]!!.lane to out["pill"]!!.lanes)
        assertEquals(1 to 2, out["next"]!!.lane to out["next"]!!.lanes)
        val apart = layout(Block("pill", at(13), at(13, 15)), Block("next", at(13, 25), at(14)))
        assertEquals(1, apart["next"]!!.lanes)
    }

    @Test fun aBlockRunningPastMidnightStopsAtTheEndOfTheDay() {
        val out = layout(Block("release", at(22, 30), at(24, 30)))
        assertEquals(1440, out["release"]!!.end)
    }

    @Test fun anEmptyDayHasNoBlocks() {
        assertTrue(layoutDay(emptyList<Block>(), 25, { it.s }, { it.e }).isEmpty())
    }
}
