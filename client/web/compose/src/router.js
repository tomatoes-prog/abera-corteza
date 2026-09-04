import Router from 'vue-router'
import routes from './views/routes'

const router = new Router({
  mode: 'history',
  routes,
})

// Add global error handler for navigation errors
router.onError(() => undefined)

export default router
