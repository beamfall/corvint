// Router definitions for the fixture admin app (ui-router state style).
angular.module('admin').config(function ($stateProvider) {
  $stateProvider
    .state('app', {
      abstract: true,
      url: '/'
    })
    .state('app.home', {
      url: 'home',
      template: '<a data-testid="nav-teesheets" href="#!/clubs/1/teesheets">Tee sheets</a>'
    })
    .state('app.clubs', {
      url: 'clubs/:clubId',
      data: { permissions: ['club.view'] }
    })
    .state('app.clubs.teesheets', {
      url: '/teesheets?date',
      templateUrl: 'app/components/teesheet.html'
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
