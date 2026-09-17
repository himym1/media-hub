import { useState } from 'react'
import { User } from 'lucide-react'
import { embyPrimaryImageURL, type EmbyPerson } from '../../shared/api/mediaHub'
import { formatPersonRole, personInitials } from './libraryCast'

export interface LibraryCastGalleryProps {
  people?: EmbyPerson[]
  onSelectPerson?: (name: string) => void
}

export function LibraryCastGallery({ people, onSelectPerson }: LibraryCastGalleryProps) {
  if (!people || people.length === 0) {
    return null
  }

  return (
    <section aria-label="演职人员" className="library-cast-section">
      <div className="library-cast-heading">
        <h3 className="library-cast-title">演职人员</h3>
        <span className="library-cast-count">{people.length} 位</span>
      </div>
      <div
        aria-label="演职人员横向画廊"
        className="library-cast-track"
        tabIndex={0}
      >
        {people.map((person, index) => (
          <CastCard
            key={`${person.id}-${person.role ?? ''}-${index}`}
            onSelect={onSelectPerson}
            person={person}
          />
        ))}
      </div>
    </section>
  )
}

function CastCard({
  person,
  onSelect,
}: {
  person: EmbyPerson
  onSelect?: (name: string) => void
}) {
  const [imgFailed, setImgFailed] = useState(!person.primaryImageTag && !person.id)
  const roleText = formatPersonRole(person)
  const isDirector = person.type?.trim().toLowerCase() === 'director'
  const actionTitle = onSelect ? `${person.name} (${roleText}) · 点击在媒体库中查看其作品` : `${person.name} (${roleText})`

  return (
    <button
      className="cast-card-btn"
      onClick={() => onSelect?.(person.name)}
      title={actionTitle}
      type="button"
    >
      <div className="cast-avatar-wrap">
        {!imgFailed ? (
          <img
            alt=""
            className="cast-avatar-img"
            decoding="async"
            height={68}
            loading="lazy"
            onError={() => setImgFailed(true)}
            src={embyPrimaryImageURL(person.id)}
            width={68}
          />
        ) : (
          <div aria-hidden="true" className="cast-avatar-fallback">
            <span className="cast-avatar-initials">{personInitials(person.name)}</span>
            <User className="cast-avatar-icon" size={14} />
          </div>
        )}
        {isDirector ? (
          <span aria-hidden="true" className="cast-director-badge" title="导演">
            导
          </span>
        ) : null}
      </div>
      <div className="cast-card-info">
        <span className="cast-card-name">{person.name}</span>
        <span className="cast-card-role">{roleText}</span>
      </div>
    </button>
  )
}
