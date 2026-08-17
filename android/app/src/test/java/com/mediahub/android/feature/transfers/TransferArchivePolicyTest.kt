package com.mediahub.android.feature.transfers

import com.mediahub.android.core.network.TransferJob
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class TransferArchivePolicyTest {
    @Test
    fun onlySettledNonRetryableJobsCanBeArchived() {
        assertTrue(canArchiveTransfer(job(state = "completed", retryable = false)))
        assertTrue(canArchiveTransfer(job(state = "failed", retryable = false)))
        assertFalse(canArchiveTransfer(job(state = "failed", retryable = true)))
        assertFalse(canArchiveTransfer(job(state = "needs_attention", retryable = false)))
        assertFalse(canArchiveTransfer(job(state = "transferring", retryable = false)))
    }

    @Test
    fun detailClosesOnlyForInitializedMissingOrMatchingCompletedTarget() {
        val item = job(state = "completed", retryable = false)
        assertFalse(shouldCloseMissingTransferDetail("job-1", TransferUiState()))
        assertFalse(shouldCloseMissingTransferDetail("job-1", TransferUiState(initialized = true, jobs = listOf(item))))
        assertTrue(shouldCloseMissingTransferDetail("job-1", TransferUiState(initialized = true)))
        assertFalse(shouldCloseCompletedTransferDetail("job-2", "job-1"))
        assertFalse(shouldCloseCompletedTransferDetail("job-1", null))
        assertTrue(shouldCloseCompletedTransferDetail("job-1", "job-1"))
    }

    private fun job(state: String, retryable: Boolean) = TransferJob(
        id = "job-1",
        title = "Movie",
        year = 2026,
        season = 0,
        mediaType = "movie",
        tmdbId = "1",
        source = "source",
        state = state,
        errorCode = null,
        errorMessage = null,
        retryable = retryable,
        archived = false,
        createdAt = "2026-01-01T00:00:00Z",
        updatedAt = "2026-01-01T00:00:00Z",
    )
}
