$(function() {

    $('#side-menu').metisMenu();

});

//Loads the correct sidebar on window load,
//collapses the sidebar on window resize.
$(function() {
    $(window).bind("load resize", function() {
        width = (this.window.innerWidth > 0) ? this.window.innerWidth : this.screen.width;
        if (width < 768) {
            $('div.sidebar-collapse').addClass('collapse')
        } else {
            $('div.sidebar-collapse').removeClass('collapse')
        }
    })
});

var app = angular.module('app',['ngRoute'], function() {
});

app.config(['$routeProvider', '$locationProvider',
    function ($routeProvider, $locationProvider) {
        $routeProvider.when('/datas', {templateUrl: 'datas.html',controller: 'DatasCtrl'})
        .when('/', {templateUrl: 'dashboard.html',controller: 'DashboardCtrl'})
        .otherwise({redirectTo: '/'});
        // configure html5 to get links working on jsfiddle
        $locationProvider.html5Mode(true);
}]);