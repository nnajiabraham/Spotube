import type { ReactNode } from 'react'

export type TabItem<T extends string> = { value: T; label: ReactNode }

/** Segmented control. The active tab is ink; values are strings ('' for "all"). */
export function Tabs<T extends string>({
  items,
  value,
  onChange,
  label,
  size = 'md',
}: {
  items: Array<TabItem<T>>
  value: T
  onChange: (value: T) => void
  label: string
  size?: 'sm' | 'md'
}) {
  return (
    <div className="inline-flex flex-wrap rounded-lg border border-gray-200 bg-white p-1" role="tablist" aria-label={label}>
      {items.map((tab) => {
        const active = tab.value === value
        return (
          <button
            key={tab.value}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => onChange(tab.value)}
            className={`inline-flex items-center rounded-md font-medium transition-colors ${
              size === 'sm' ? 'px-2.5 py-1 text-xs' : 'px-3 py-1.5 text-sm'
            } ${active ? 'bg-ink text-white' : 'text-gray-600 hover:bg-gray-100 hover:text-ink'}`}
          >
            {tab.label}
          </button>
        )
      })}
    </div>
  )
}
