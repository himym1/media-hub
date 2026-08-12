package com.mediahub.android.app

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import com.mediahub.android.data.MediaHubRepository
import com.mediahub.android.feature.auth.AuthViewModel
import com.mediahub.android.feature.library.LibraryViewModel
import com.mediahub.android.feature.operations.OperationsViewModel
import com.mediahub.android.feature.search.SearchViewModel
import com.mediahub.android.feature.services.ServicesViewModel
import com.mediahub.android.feature.subscriptions.SubscriptionViewModel
import com.mediahub.android.feature.transfers.TransferViewModel

class MediaHubViewModelFactory(
    private val repository: MediaHubRepository,
) : ViewModelProvider.Factory {
    @Suppress("UNCHECKED_CAST")
    override fun <T : ViewModel> create(modelClass: Class<T>): T = when {
        modelClass.isAssignableFrom(AppViewModel::class.java) -> AppViewModel(repository) as T
        modelClass.isAssignableFrom(AuthViewModel::class.java) -> AuthViewModel(repository) as T
        modelClass.isAssignableFrom(SearchViewModel::class.java) -> SearchViewModel(repository) as T
        modelClass.isAssignableFrom(TransferViewModel::class.java) -> TransferViewModel(repository) as T
        modelClass.isAssignableFrom(LibraryViewModel::class.java) -> LibraryViewModel(repository) as T
        modelClass.isAssignableFrom(ServicesViewModel::class.java) -> ServicesViewModel(repository) as T
        modelClass.isAssignableFrom(OperationsViewModel::class.java) -> OperationsViewModel(repository) as T
        modelClass.isAssignableFrom(SubscriptionViewModel::class.java) -> SubscriptionViewModel(repository) as T
        else -> throw IllegalArgumentException("Unknown ViewModel class")
    }
}
