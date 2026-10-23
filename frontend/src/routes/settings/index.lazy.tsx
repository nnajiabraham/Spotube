import { createLazyFileRoute, Link } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { settingsAPI, type AppSetting } from '../../lib/api/index'
import { QuotaPanel } from '../../components/QuotaPanel'
import { Button, Card, Notice, Page, Pill, SectionTitle, ServiceIcon } from '../../components/ui'
import { inputClass, sectionLinkClass } from '../../lib/format'

const groups = [
  { key: 'sync', title: 'Sync' },
  { key: 'quota', title: 'YouTube quota' },
  { key: 'backups', title: 'Backups' },
]

function SourceBadge({ setting }: { setting: AppSetting }) {
  switch (setting.source) {
    case 'env':
      return <Pill title={`Set by ${setting.env} in backend/.env`}>.env</Pill>
    case 'runtime':
      return (
        <Pill
          tone="yellow"
          title={`backend/.env sets ${setting.env}=${setting.env_value}; that returns when the server restarts`}
        >
          until restart
        </Pill>
      )
    case 'database':
      return <Pill tone="green">saved</Pill>
    default:
      return <Pill>default</Pill>
  }
}

function SettingRow({
  setting,
  onSave,
  saving,
}: {
  setting: AppSetting
  onSave: (value: boolean | number) => void
  saving: boolean
}) {
  const [draft, setDraft] = useState(setting.value)
  useEffect(() => setDraft(setting.value), [setting.value])
  const id = `setting-${setting.key}`

  return (
    <div className="flex flex-wrap items-start justify-between gap-x-6 gap-y-3 px-5 py-4">
      <div className="min-w-0 max-w-xl flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <label htmlFor={id} className="text-sm font-medium text-ink">
            {setting.label}
          </label>
          <SourceBadge setting={setting} />
        </div>
        <p className="mt-0.5 text-sm text-gray-500">{setting.description}</p>
        {setting.env_pinned && (
          <p className="mt-1 text-xs text-gray-400">
            From <code>{setting.env}</code> in backend/.env ({setting.env_value}
            ). Changes here last until the server restarts; edit .env to keep them.
          </p>
        )}
      </div>
      {setting.kind === 'bool' ? (
        <button
          id={id}
          type="button"
          role="switch"
          aria-checked={setting.value === 'true'}
          disabled={saving}
          onClick={() => onSave(setting.value !== 'true')}
          className={`relative mt-1 inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors disabled:opacity-50 ${
            setting.value === 'true' ? 'bg-ink' : 'bg-gray-300'
          }`}
        >
          <span
            className={`inline-block h-4 w-4 rounded-full bg-white shadow transition-transform ${
              setting.value === 'true' ? 'translate-x-4.5' : 'translate-x-0.5'
            }`}
          />
        </button>
      ) : (
        <form
          className="flex shrink-0 items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            onSave(Number(draft))
          }}
        >
          <input
            id={id}
            type="number"
            min={setting.min}
            max={setting.max || undefined}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            className={`${inputClass} !w-28 text-right tabular-nums`}
          />
          <Button type="submit" size="sm" disabled={draft === setting.value || draft === ''} loading={saving}>
            Save
          </Button>
        </form>
      )}
    </div>
  )
}

function SettingsPage() {
  const queryClient = useQueryClient()
  const settings = useQuery({
    queryKey: ['settings'],
    queryFn: settingsAPI.list,
  })
  const [error, setError] = useState<string | null>(null)
  const save = useMutation({
    mutationFn: ({ key, value }: { key: string; value: boolean | number }) => settingsAPI.update({ [key]: value }),
    onMutate: () => setError(null),
    onSuccess: (items) => {
      queryClient.setQueryData(['settings'], items)
      for (const key of ['quota', 'dashboard-stats', 'schedule-recommendation', 'quota-estimate']) {
        queryClient.invalidateQueries({ queryKey: [key] })
      }
    },
    onError: (err: Error) => setError(err.message),
  })

  return (
    <Page
      title="Settings"
      width="narrow"
      description={
        <>
          Values set in <code>backend/.env</code> are the defaults at startup; changing them here lasts until the server
          restarts. Everything else is saved in Spotube's database.
        </>
      }
    >
      {error && (
        <Notice tone="error" className="mb-4" onDismiss={() => setError(null)}>
          {error}
        </Notice>
      )}
      {settings.isLoading ? (
        <p className="text-sm text-gray-500" role="status">
          Loading settings…
        </p>
      ) : settings.error ? (
        <Notice tone="error">Couldn't load settings: {settings.error.message}</Notice>
      ) : (
        groups.map((group) => {
          const items = settings.data?.filter((s) => s.group === group.key) ?? []
          if (items.length === 0) return null
          return (
            <section key={group.key} className="mb-8">
              <SectionTitle>{group.title}</SectionTitle>
              {group.key === 'quota' && <QuotaPanel className="mb-3" />}
              <Card className="divide-y divide-gray-100">
                {items.map((s) => (
                  <SettingRow
                    key={s.key}
                    setting={s}
                    saving={save.isPending && save.variables?.key === s.key}
                    onSave={(value) => save.mutate({ key: s.key, value })}
                  />
                ))}
              </Card>
            </section>
          )
        })
      )}

      <section>
        <SectionTitle>Playlist libraries</SectionTitle>
        <Card className="flex flex-wrap gap-6 px-5 py-4">
          <Link to="/settings/spotify" className={`${sectionLinkClass} inline-flex items-center gap-2 text-sm`}>
            <ServiceIcon service="spotify" className="h-4 w-4" /> Spotify playlists →
          </Link>
          <Link to="/settings/youtube" className={`${sectionLinkClass} inline-flex items-center gap-2 text-sm`}>
            <ServiceIcon service="youtube" className="h-4 w-4" /> YouTube playlists →
          </Link>
        </Card>
      </section>
    </Page>
  )
}

export const Route = createLazyFileRoute('/settings/')({
  component: SettingsPage,
})
