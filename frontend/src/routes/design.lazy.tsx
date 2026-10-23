import { createLazyFileRoute } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'
import { Inbox, Search, Trash2 } from 'lucide-react'
import {
  Button,
  Card,
  CountBadge,
  Dialog,
  Drawer,
  EmptyState,
  Notice,
  Page,
  Pagination,
  Pill,
  QueueStatusPill,
  ScoreBadge,
  SectionTitle,
  ServiceIcon,
  SpotubeMark,
  Tabs,
  TrackRow,
} from '../components/ui'
import { inputClass, linkClass } from '../lib/format'

const swatches = [
  { name: 'ink', hex: '#0f0f0f', className: 'bg-ink', use: 'Header, primary buttons, headings' },
  { name: 'spotify', hex: '#1ed760', className: 'bg-spotify', use: 'Spotify, success, synced' },
  { name: 'spotify-700', hex: '#117a37', className: 'bg-spotify-700', use: 'Green text on white' },
  { name: 'spotify-50', hex: '#ebfbf1', className: 'bg-spotify-50', use: 'Success backgrounds' },
  { name: 'youtube', hex: '#ff0033', className: 'bg-youtube', use: 'YouTube, errors, destructive' },
  { name: 'youtube-700', hex: '#c4001f', className: 'bg-youtube-700', use: 'Red text on white' },
  { name: 'youtube-50', hex: '#fff0f2', className: 'bg-youtube-50', use: 'Error backgrounds' },
  { name: 'amber-400', hex: '#fbbf24', className: 'bg-amber-400', use: 'Needs you: review, pending' },
]

const breakdown = {
  total: 0.62,
  title: 0.8,
  artist: 1,
  artist_known: true,
  duration_diff_sec: 74,
  adjustments: [
    { reason: 'length differs by 74s', delta: -0.08 },
    { reason: 'only one is live', delta: -0.2 },
    { reason: 'YouTube Music upload', delta: 0.03 },
  ],
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="mb-10">
      <SectionTitle>{title}</SectionTitle>
      {children}
    </section>
  )
}

function DesignPage() {
  const [tab, setTab] = useState<'a' | 'b' | 'c'>('a')
  const [page, setPage] = useState(2)
  const [dialog, setDialog] = useState(false)
  const [drawer, setDrawer] = useState(false)

  return (
    <Page
      title="Design system"
      description={
        <>
          Spotube's palette and components. Rules live in <code className="text-ink">docs/design-system.md</code>.
        </>
      }
    >
      <Section title="Palette">
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {swatches.map((s) => (
            <Card key={s.name} className="overflow-hidden">
              <div className={`h-16 ${s.className} ${s.name.endsWith('50') ? 'border-b border-gray-200' : ''}`} />
              <div className="p-3 text-sm">
                <div className="font-medium text-ink">{s.name}</div>
                <div className="font-mono text-xs text-gray-500">{s.hex}</div>
                <div className="mt-1 text-xs text-gray-500">{s.use}</div>
              </div>
            </Card>
          ))}
        </div>
        <div className="mt-4 flex items-center gap-4 rounded-lg bg-ink p-4 text-white">
          <SpotubeMark className="h-10 w-10 rounded-lg ring-1 ring-white/15" />
          <div>
            <div className="font-semibold">Spotube</div>
            <div className="text-sm text-gray-400">Green out, red back, on ink.</div>
          </div>
          <div className="ml-auto flex gap-3">
            <ServiceIcon service="spotify" className="h-6 w-6" />
            <ServiceIcon service="youtube" className="h-6 w-6" />
          </div>
        </div>
      </Section>

      <Section title="Buttons">
        <Card className="space-y-4 p-4">
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="primary">Primary</Button>
            <Button variant="secondary">Secondary</Button>
            <Button variant="ghost">Ghost</Button>
            <Button variant="spotify">Connect Spotify</Button>
            <Button variant="danger" icon={<Trash2 className="h-4 w-4" />}>
              Delete
            </Button>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="primary" size="sm">
              Use this
            </Button>
            <Button variant="secondary" size="sm" icon={<Search className="h-3.5 w-3.5" />}>
              Find on YouTube
            </Button>
            <Button variant="primary" loading>
              Saving
            </Button>
            <Button variant="secondary" disabled>
              Disabled
            </Button>
          </div>
        </Card>
      </Section>

      <Section title="Status">
        <Card className="space-y-3 p-4">
          <div className="flex flex-wrap gap-2">
            {['review', 'pending', 'running', 'done', 'skipped', 'error'].map((s) => (
              <QueueStatusPill key={s} status={s} />
            ))}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Pill tone="ink">You decided</Pill>
            <Pill>Auto</Pill>
            <ScoreBadge value={0.97} />
            <ScoreBadge value={0.72} />
            <ScoreBadge value={0.31} />
            <span className="text-sm text-gray-600">
              Review
              <CountBadge count={7} />
            </span>
          </div>
        </Card>
      </Section>

      <Section title="Messages">
        <div className="space-y-2">
          <Notice tone="success">Added “Fearless” to YT Worship. Spotube will remember this match.</Notice>
          <Notice tone="warning">Still no confident match for “Pelo Negro”.</Notice>
          <Notice tone="error">YouTube quota exceeded for today.</Notice>
          <Notice tone="info">Repeating a search within 30 minutes is free.</Notice>
        </div>
      </Section>

      <Section title="Navigation">
        <Card className="space-y-4 p-4">
          <Tabs
            label="Example"
            value={tab}
            onChange={setTab}
            items={[
              { value: 'a', label: 'All' },
              { value: 'b', label: 'Paired' },
              { value: 'c', label: 'Not on YouTube' },
            ]}
          />
          <p className="text-sm text-gray-600">
            Text links look like <a className={linkClass} href="#design">this</a>.
          </p>
          <div className="overflow-hidden rounded-lg border border-gray-200">
            <Pagination page={page} totalPages={5} totalItems={120} noun="track" onPage={setPage} />
          </div>
        </Card>
      </Section>

      <Section title="Tracks">
        <Card className="divide-y divide-gray-100 p-2">
          <TrackRow
            service="spotify"
            track={{ id: '1', title: 'Fearless', artist: 'Sounds Of Salem', album: 'Fearless', duration_ms: 300000 }}
          />
          <TrackRow
            service="youtube"
            track={{ id: 'x', title: 'Fearless (Live)', artist: 'Sounds Of Salem', channel: 'Salem Worship', duration_ms: 374000 }}
            trailing={
              <>
                <ScoreBadge value={0.62} breakdown={breakdown} />
                <Button size="sm" variant="primary">
                  Use this
                </Button>
              </>
            }
          />
          <TrackRow
            service="youtube"
            highlight
            track={{ id: 'y', title: 'Fearless', artist: 'Sounds Of Salem', channel: 'Sounds Of Salem - Topic', duration_ms: 301000 }}
            trailing={<ScoreBadge value={0.98} />}
          />
        </Card>
        <p className="mt-2 text-xs text-gray-500">Click the 62% badge to see how a score is built.</p>
      </Section>

      <Section title="Inputs">
        <Card className="flex flex-wrap gap-2 p-4">
          <input className={`${inputClass} max-w-xs`} placeholder="Search YouTube" aria-label="Example input" />
          <select className={`${inputClass} !w-auto`} aria-label="Example select">
            <option>Both directions</option>
          </select>
        </Card>
      </Section>

      <Section title="Empty states and overlays">
        <div className="grid gap-4 md:grid-cols-2">
          <Card>
            <EmptyState icon={<Inbox />} title="Nothing to review" action={<Button variant="primary">Sync now</Button>}>
              Every track either synced or has a decision.
            </EmptyState>
          </Card>
          <Card className="flex items-center justify-center gap-2 p-6">
            <Button onClick={() => setDialog(true)}>Open dialog</Button>
            <Button onClick={() => setDrawer(true)}>Open drawer</Button>
          </Card>
        </div>
      </Section>

      {dialog && (
        <Dialog
          title="Find “Fearless” on YouTube"
          subtitle="Sounds Of Salem"
          onClose={() => setDialog(false)}
          footer={
            <Button variant="primary" onClick={() => setDialog(false)}>
              Done
            </Button>
          }
        >
          Dialogs hold short tasks such as picking a track. Escape or a click outside closes them.
        </Dialog>
      )}
      {drawer && (
        <Drawer title="Sync item" subtitle="Worship → YT Worship" onClose={() => setDrawer(false)}>
          Drawers show details next to a list without leaving it.
        </Drawer>
      )}
    </Page>
  )
}

export const Route = createLazyFileRoute('/design')({
  component: DesignPage,
})
