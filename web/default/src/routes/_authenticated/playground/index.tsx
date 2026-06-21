/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useEffect, useState } from 'react'
import { LoaderCircle } from 'lucide-react'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { isSidebarModuleEnabled } from '@/lib/nav-modules'
import { Main } from '@/components/layout'

export const Route = createFileRoute('/_authenticated/playground/')({
  beforeLoad: () => {
    if (!isSidebarModuleEnabled('chat', 'playground')) {
      throw redirect({ to: '/dashboard' })
    }
  },
  component: PlaygroundBridgePage,
})

const reloadGuardKey = 'newapi_playground_bridge_reload'

function PlaygroundBridgePage() {
  const [reloadFailed, setReloadFailed] = useState(false)

  useEffect(() => {
    if (typeof window === 'undefined') return
    const target = `/playground/${window.location.search}${window.location.hash}`

    if (window.sessionStorage.getItem(reloadGuardKey) === '1') {
      window.sessionStorage.removeItem(reloadGuardKey)
      setReloadFailed(true)
      return
    }

    window.sessionStorage.setItem(reloadGuardKey, '1')
    window.location.replace(target)
  }, [])

  return (
    <Main className='flex min-h-[60vh] items-center justify-center px-6'>
      <div className='flex max-w-md flex-col items-center gap-4 text-center'>
        <LoaderCircle className='size-8 animate-spin text-primary' />
        <div className='space-y-2'>
          <h1 className='text-xl font-semibold'>Opening Image Playground</h1>
          <p className='text-sm text-muted-foreground'>
            {reloadFailed
              ? 'The browser stayed in the console shell. Use the direct link below to open the standalone playground.'
              : 'Switching from the console shell to the standalone playground...'}
          </p>
        </div>
        {reloadFailed ? (
          <div className='flex flex-col gap-2 text-sm'>
            <a className='text-primary underline underline-offset-4' href='/playground/'>
              Open `/playground`
            </a>
            <a
              className='text-muted-foreground underline underline-offset-4'
              href='/console/playground'
            >
              Open built-in chat playground
            </a>
          </div>
        ) : null}
      </div>
    </Main>
  )
}
