import { expect } from 'chai'
import CAppSelector from 'corteza-webapp-one/src/components/CAppSelector'

const methods = CAppSelector.methods

const context = (translations = {}) => ({
  $t: key => translations[key] || key,
  applicationRoute: methods.applicationRoute,
})

describe('CAppSelector.vue', () => {
  const builtIn = (url, name = 'Canonical name') => ({
    name,
    unify: { url },
  })

  it('uses localized labels for built-in applications', () => {
    const vm = context({ 'applications.compose': 'Espacios de trabajo' })

    expect(methods.displayName.call(vm, builtIn('compose/'))).to.equal('Espacios de trabajo')
  })

  it('preserves an explicitly configured application name', () => {
    const vm = context({ 'applications.compose': 'Espacios de trabajo' })
    const app = builtIn('compose/')
    app.unify.name = 'Portal comercial'

    expect(methods.displayName.call(vm, app)).to.equal('Portal comercial')
  })

  it('falls back to the canonical name for custom applications', () => {
    const vm = context()

    expect(methods.displayName.call(vm, builtIn('custom/', 'Mi aplicación'))).to.equal('Mi aplicación')
  })

  it('keeps Jitsi available and localizes its presentation label', () => {
    const vm = context({ 'applications.jitsi': 'Videoconferencias Jitsi' })

    expect(methods.displayName.call(vm, builtIn('/bridge/jitsi/?room=demo'))).to.equal('Videoconferencias Jitsi')
  })

  it('searches localized and canonical application names', () => {
    const apps = [builtIn('compose/', 'Namespaces'), builtIn('admin/', 'Admin Area')]
    const vm = {
      appList: apps,
      query: 'espacios',
      displayName: app => app.unify.url === 'compose/' ? 'Espacios de trabajo' : 'Configuración',
    }

    expect(CAppSelector.computed.filteredApps.call(vm)).to.deep.equal([apps[0]])

    vm.query = 'admin'
    expect(CAppSelector.computed.filteredApps.call(vm)).to.deep.equal([apps[1]])
  })
})
