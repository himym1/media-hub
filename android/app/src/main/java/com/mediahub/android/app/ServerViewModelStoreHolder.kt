package com.mediahub.android.app

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelStore
import androidx.lifecycle.ViewModelStoreOwner

class ServerViewModelStoreHolder : ViewModel() {
    private var generation: Long? = null
    private var owner: ScopedViewModelStoreOwner? = null

    fun ownerFor(serverGeneration: Long): ViewModelStoreOwner {
        if (generation != serverGeneration || owner == null) {
            clearServerScope()
            generation = serverGeneration
            owner = ScopedViewModelStoreOwner()
        }
        return requireNotNull(owner)
    }

    fun clearServerScope() {
        owner?.viewModelStore?.clear()
        owner = null
        generation = null
    }

    override fun onCleared() {
        clearServerScope()
    }
}

private class ScopedViewModelStoreOwner : ViewModelStoreOwner {
    override val viewModelStore = ViewModelStore()
}
