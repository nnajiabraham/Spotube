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
    <div className="inline-flex flex-wrap gap-0.5 rounded-md bg-gray-100 p-0.5" role="tablist" aria-label={label}>
      {items.map((tab) => {
        const active = tab.value === value
        return (
          <button
            key={tab.value}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => onChange(tab.value)}
            className={`inline-flex items-center rounded font-medium transition-colors ${
              size === 'sm' ? 'px-2.5 py-1 text-xs' : 'px-3 py-1 text-sm'
            } ${active ? 'bg-white text-ink shadow-[0_0_0_1px_var(--color-gray-200)]' : 'text-gray-500 hover:text-ink'}`}
          >
            {tab.label}
          </button>
        )
      })}
    </div>
  )
}
