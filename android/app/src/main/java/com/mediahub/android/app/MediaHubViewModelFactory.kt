package com.mediahub.android.app

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import com.mediahub.android.data.MediaHubRepository
import com.mediahub.android.feature.auth.AuthViewModel
import com.mediahub.android.feature.library.LibraryDetailViewModel
import com.mediahub.android.feature.library.LibraryViewModel
import com.mediahub.android.feature.operations.ArchiveViewModel
import com.mediahub.android.feature.operations.Drive115ViewModel
import com.mediahub.android.feature.operations.LocalUploadViewModel
import com.mediahub.android.feature.search.SearchViewModel
import com.mediahub.android.feature.services.ServicesViewModel
import com.mediahub.android.feature.subscriptions.SubscriptionViewModel
import com.mediahub.android.feature.subtitles.RemoteSubtitleViewModel
import com.mediahub.android.feature.transfers.TransferViewModel

class MediaHubViewModelFactory(
    private val repository: MediaHubRepository,
    private val updateNotifier: AndroidUpdateNotifier = AndroidUpdateNotifier.None,
    private val evictItemCaches: (String) -> Unit = {},
    private val updatePromptStore: AndroidUpdatePromptStore? = null,
) : ViewModelProvider.Factory {
    @Suppress("UNCHECKED_CAST")
    override fun <T : ViewModel> create(modelClass: Class<T>): T = when {
        modelClass.isAssignableFrom(AppViewModel::class.java) -> AppViewModel(repository, updateNotifier, updatePromptStore) as T
        modelClass.isAssignableFrom(AuthViewModel::class.java) -> AuthViewModel(repository) as T
        modelClass.isAssignableFrom(SearchViewModel::class.java) -> SearchViewModel(repository) as T
        modelClass.isAssignableFrom(TransferViewModel::class.java) -> TransferViewModel(repository) as T
        modelClass.isAssignableFrom(LibraryViewModel::class.java) -> LibraryViewModel(repository) as T
        modelClass.isAssignableFrom(LibraryDetailViewModel::class.java) -> LibraryDetailViewModel(repository, evictItemCaches) as T
        modelClass.isAssignableFrom(ServicesViewModel::class.java) -> ServicesViewModel(repository, updateNotifier) as T
        modelClass.isAssignableFrom(Drive115ViewModel::class.java) -> Drive115ViewModel(repository) as T
        modelClass.isAssignableFrom(LocalUploadViewModel::class.java) -> LocalUploadViewModel(repository) as T
        modelClass.isAssignableFrom(ArchiveViewModel::class.java) -> ArchiveViewModel(repository) as T
        modelClass.isAssignableFrom(SubscriptionViewModel::class.java) -> SubscriptionViewModel(repository) as T
        modelClass.isAssignableFrom(RemoteSubtitleViewModel::class.java) -> RemoteSubtitleViewModel(repository) as T
        else -> throw IllegalArgumentException("Unknown ViewModel class")
    }
}
