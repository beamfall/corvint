// Router definitions for the fixture admin app (ui-router state style).
angular.module('admin').config(function ($stateProvider) {
  $stateProvider
    .state('app', {
      abstract: true,
      url: '/',
      data: { permissions: ['session'] }
    })
    .state('app.home', {
      url: 'home'
    })
    .state('app.clubs', {
      url: 'clubs/:clubId',
      data: { permissions: ['club.view'] }
    })
    .state('app.clubs.teesheets', {
      url: '/teesheets?date',
      data: { permissions: ['teesheet.view'], flags: ['new_teesheet'] }
    })
    .state('app.clubs.settings', {
      url: '/settings'
    })
    .state('app.clubs.members', {
      url: '/members'
    })
    .state('members2', {
      parent: 'app.clubs',
      url: '/members'
    })
    .state({
      name: 'reports',
      parent: 'app.clubs',
      url: '/reports/{reportId:int}'
    })
    .state('login', {
      url: '^/login'
    })
    .state('app.dynamic', {
      url: buildUrl('dynamic')
    })
    .state('orphan.child', {
      url: '/child'
    });
});
