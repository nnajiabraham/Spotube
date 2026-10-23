import { createLazyFileRoute } from '@tanstack/react-router'
import { PlaylistLibrary } from '../../components/PlaylistLibrary'
import { Card, Page } from '../../components/ui'

function YouTubePlaylists() {
  return (
    <Page
      title="YouTube playlists"
      width="narrow"
      description="Everything in the connected YouTube account. Back up any playlist here; synced ones show their pair."
    >
      <Card className="p-4">
        <PlaylistLibrary service="youtube" />
      </Card>
    </Page>
  )
}

export const Route = createLazyFileRoute('/settings/youtube')({
  component: YouTubePlaylists,
})
