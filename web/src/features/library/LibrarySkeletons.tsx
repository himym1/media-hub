/**
 * Infuse / Apple TV 风格的微光骨架屏组件 (Shimmer Skeletons)
 */

export function LibraryPosterGridSkeleton({ count = 12 }: { count?: number }) {
  return (
    <div
      aria-busy="true"
      aria-label="正在加载媒体海报..."
      className="library-poster-grid skeleton-grid"
    >
      {Array.from({ length: count }).map((_, index) => (
        <div className="library-poster-card skeleton-card" key={index}>
          <div className="library-poster-frame skeleton-shimmer" />
          <div className="skeleton-line skeleton-title skeleton-shimmer" />
          <div className="skeleton-line skeleton-subtitle skeleton-shimmer" />
        </div>
      ))}
    </div>
  )
}

export function LibraryDetailSkeleton() {
  return (
    <div aria-busy="true" aria-label="正在加载媒体详情..." className="library-detail-skeleton">
      {/* 顶部全景氛围背景骨架 */}
      <div className="skeleton-backdrop skeleton-shimmer" />

      {/* 主体信息区 */}
      <div className="library-detail-hero">
        {/* 2:3 黄金海报骨架 */}
        <div className="library-detail-poster skeleton-shimmer" />

        {/* 右侧文本与规格骨架 */}
        <div className="library-detail-hero-copy">
          <div className="skeleton-line skeleton-heading skeleton-shimmer" />
          <div className="skeleton-pill-row">
            <span className="skeleton-pill skeleton-shimmer" style={{ width: '68px' }} />
            <span className="skeleton-pill skeleton-shimmer" style={{ width: '84px' }} />
            <span className="skeleton-pill skeleton-shimmer" style={{ width: '56px' }} />
            <span className="skeleton-pill skeleton-shimmer" style={{ width: '48px' }} />
          </div>

          <div className="skeleton-actions-row">
            <div className="skeleton-btn skeleton-shimmer" style={{ width: '130px', height: '44px' }} />
            <div className="skeleton-btn skeleton-shimmer" style={{ width: '100px', height: '44px' }} />
          </div>

          <div className="skeleton-facts-grid">
            <div className="skeleton-fact-cell skeleton-shimmer" />
            <div className="skeleton-fact-cell skeleton-shimmer" />
            <div className="skeleton-fact-cell skeleton-shimmer" />
          </div>

          <div className="skeleton-overview-block">
            <div className="skeleton-line skeleton-shimmer" style={{ width: '92%' }} />
            <div className="skeleton-line skeleton-shimmer" style={{ width: '98%' }} />
            <div className="skeleton-line skeleton-shimmer" style={{ width: '75%' }} />
          </div>
        </div>
      </div>

      {/* 演职人员横向画廊骨架 */}
      <div className="skeleton-cast-section">
        <div className="skeleton-line skeleton-shimmer" style={{ width: '120px', height: '18px', marginBottom: '14px' }} />
        <div className="skeleton-cast-row">
          {Array.from({ length: 6 }).map((_, index) => (
            <div className="skeleton-cast-card" key={index}>
              <div className="skeleton-avatar skeleton-shimmer" />
              <div className="skeleton-line skeleton-shimmer" style={{ width: '54px', height: '12px', marginTop: '8px' }} />
              <div className="skeleton-line skeleton-shimmer" style={{ width: '40px', height: '10px', marginTop: '4px' }} />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export function LibraryEpisodesSkeleton() {
  return (
    <div aria-busy="true" aria-label="正在加载剧集分集..." className="library-episodes-skeleton">
      {/* 待播卡片骨架 */}
      <div className="skeleton-up-next skeleton-shimmer" />

      {/* 分季药丸栏骨架 */}
      <div className="skeleton-season-tabs">
        <span className="skeleton-pill skeleton-shimmer" style={{ width: '96px', height: '34px', borderRadius: '9999px' }} />
        <span className="skeleton-pill skeleton-shimmer" style={{ width: '96px', height: '34px', borderRadius: '9999px' }} />
        <span className="skeleton-pill skeleton-shimmer" style={{ width: '96px', height: '34px', borderRadius: '9999px' }} />
      </div>

      {/* 16:9 画幅单集剧照列表骨架 */}
      <div className="skeleton-episode-card-list">
        {Array.from({ length: 4 }).map((_, index) => (
          <div className="skeleton-episode-item" key={index}>
            <div className="skeleton-still-thumb skeleton-shimmer" />
            <div className="skeleton-episode-body">
              <div className="skeleton-line skeleton-shimmer" style={{ width: '38%', height: '16px' }} />
              <div className="skeleton-line skeleton-shimmer" style={{ width: '88%', height: '12px', marginTop: '8px' }} />
              <div className="skeleton-line skeleton-shimmer" style={{ width: '65%', height: '12px', marginTop: '4px' }} />
              <div className="skeleton-episode-footer">
                <div className="skeleton-line skeleton-shimmer" style={{ width: '70px', height: '12px' }} />
                <div className="skeleton-btn skeleton-shimmer" style={{ width: '80px', height: '32px' }} />
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
