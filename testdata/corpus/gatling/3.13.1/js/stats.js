var stats = {
    type: "GROUP",
name: "All Requests",
path: "",
pathFormatted: "group_missing-name--1146707516",
stats: {
    "name": "All Requests",
    "numberOfRequests": {
        "total": "102",
        "ok": "84",
        "ko": "18"
    },
    "minResponseTime": {
        "total": "0",
        "ok": "0",
        "ko": "0"
    },
    "maxResponseTime": {
        "total": "1503",
        "ok": "1503",
        "ko": "4"
    },
    "meanResponseTime": {
        "total": "89",
        "ok": "108",
        "ko": "1"
    },
    "standardDeviation": {
        "total": "353",
        "ok": "387",
        "ko": "1"
    },
    "percentiles1": {
        "total": "1",
        "ok": "1",
        "ko": "1"
    },
    "percentiles2": {
        "total": "1",
        "ok": "1",
        "ko": "2"
    },
    "percentiles3": {
        "total": "1072",
        "ok": "1214",
        "ko": "4"
    },
    "percentiles4": {
        "total": "1503",
        "ok": "1503",
        "ko": "4"
    },
    "group1": {
    "name": "t < 800 ms",
    "htmlName": "t < 800 ms",
    "count": 78,
    "percentage": 76.47058823529412
},
    "group2": {
    "name": "800 ms <= t < 1200 ms",
    "htmlName": "t >= 800 ms <br> t < 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group3": {
    "name": "t >= 1200 ms",
    "htmlName": "t >= 1200 ms",
    "count": 6,
    "percentage": 5.88235294117647
},
    "group4": {
    "name": "failed",
    "htmlName": "failed",
    "count": 18,
    "percentage": 17.647058823529413
},
    "meanNumberOfRequestsPerSecond": {
        "total": "25.5",
        "ok": "21",
        "ko": "4.5"
    }
},
contents: {
"group_outer-106111099": {
          type: "GROUP",
name: "outer",
path: "outer",
pathFormatted: "group_outer-106111099",
stats: {
    "name": "outer",
    "numberOfRequests": {
        "total": "6",
        "ok": "0",
        "ko": "6"
    },
    "minResponseTime": {
        "total": "1514",
        "ok": "-",
        "ko": "1514"
    },
    "maxResponseTime": {
        "total": "1515",
        "ok": "-",
        "ko": "1515"
    },
    "meanResponseTime": {
        "total": "1514",
        "ok": "-",
        "ko": "1514"
    },
    "standardDeviation": {
        "total": "0",
        "ok": "-",
        "ko": "0"
    },
    "percentiles1": {
        "total": "1514",
        "ok": "-",
        "ko": "1514"
    },
    "percentiles2": {
        "total": "1514",
        "ok": "-",
        "ko": "1514"
    },
    "percentiles3": {
        "total": "1515",
        "ok": "-",
        "ko": "1515"
    },
    "percentiles4": {
        "total": "1515",
        "ok": "-",
        "ko": "1515"
    },
    "group1": {
    "name": "t < 800 ms",
    "htmlName": "t < 800 ms",
    "count": 0,
    "percentage": 0.0
},
    "group2": {
    "name": "800 ms <= t < 1200 ms",
    "htmlName": "t >= 800 ms <br> t < 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group3": {
    "name": "t >= 1200 ms",
    "htmlName": "t >= 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group4": {
    "name": "failed",
    "htmlName": "failed",
    "count": 6,
    "percentage": 100.0
},
    "meanNumberOfRequestsPerSecond": {
        "total": "1.5",
        "ok": "-",
        "ko": "1.5"
    }
},
contents: {
"group_inner--with-com-178326725": {
          type: "GROUP",
name: "inner, with comma",
path: "outer / inner, with comma",
pathFormatted: "group_outer---inner---509682671",
stats: {
    "name": "inner, with comma",
    "numberOfRequests": {
        "total": "6",
        "ok": "0",
        "ko": "6"
    },
    "minResponseTime": {
        "total": "1503",
        "ok": "-",
        "ko": "1503"
    },
    "maxResponseTime": {
        "total": "1504",
        "ok": "-",
        "ko": "1504"
    },
    "meanResponseTime": {
        "total": "1503",
        "ok": "-",
        "ko": "1503"
    },
    "standardDeviation": {
        "total": "0",
        "ok": "-",
        "ko": "0"
    },
    "percentiles1": {
        "total": "1503",
        "ok": "-",
        "ko": "1503"
    },
    "percentiles2": {
        "total": "1503",
        "ok": "-",
        "ko": "1503"
    },
    "percentiles3": {
        "total": "1504",
        "ok": "-",
        "ko": "1504"
    },
    "percentiles4": {
        "total": "1504",
        "ok": "-",
        "ko": "1504"
    },
    "group1": {
    "name": "t < 800 ms",
    "htmlName": "t < 800 ms",
    "count": 0,
    "percentage": 0.0
},
    "group2": {
    "name": "800 ms <= t < 1200 ms",
    "htmlName": "t >= 800 ms <br> t < 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group3": {
    "name": "t >= 1200 ms",
    "htmlName": "t >= 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group4": {
    "name": "failed",
    "htmlName": "failed",
    "count": 6,
    "percentage": 100.0
},
    "meanNumberOfRequestsPerSecond": {
        "total": "1.5",
        "ok": "-",
        "ko": "1.5"
    }
},
contents: {
"req_get--slow--1281449658": {
        type: "REQUEST",
        name: "GET /slow",
path: "outer / inner, with comma / GET /slow",
pathFormatted: "req_outer---inner----1273628700",
stats: {
    "name": "GET /slow",
    "numberOfRequests": {
        "total": "6",
        "ok": "6",
        "ko": "0"
    },
    "minResponseTime": {
        "total": "1502",
        "ok": "1502",
        "ko": "-"
    },
    "maxResponseTime": {
        "total": "1503",
        "ok": "1503",
        "ko": "-"
    },
    "meanResponseTime": {
        "total": "1502",
        "ok": "1502",
        "ko": "-"
    },
    "standardDeviation": {
        "total": "0",
        "ok": "0",
        "ko": "-"
    },
    "percentiles1": {
        "total": "1502",
        "ok": "1502",
        "ko": "-"
    },
    "percentiles2": {
        "total": "1502",
        "ok": "1502",
        "ko": "-"
    },
    "percentiles3": {
        "total": "1503",
        "ok": "1503",
        "ko": "-"
    },
    "percentiles4": {
        "total": "1503",
        "ok": "1503",
        "ko": "-"
    },
    "group1": {
    "name": "t < 800 ms",
    "htmlName": "t < 800 ms",
    "count": 0,
    "percentage": 0.0
},
    "group2": {
    "name": "800 ms <= t < 1200 ms",
    "htmlName": "t >= 800 ms <br> t < 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group3": {
    "name": "t >= 1200 ms",
    "htmlName": "t >= 1200 ms",
    "count": 6,
    "percentage": 100.0
},
    "group4": {
    "name": "failed",
    "htmlName": "failed",
    "count": 0,
    "percentage": 0.0
},
    "meanNumberOfRequestsPerSecond": {
        "total": "1.5",
        "ok": "1.5",
        "ko": "-"
    }
}
    },"req_get--fail--1281847709": {
        type: "REQUEST",
        name: "GET /fail",
path: "outer / inner, with comma / GET /fail",
pathFormatted: "req_outer---inner----1274026751",
stats: {
    "name": "GET /fail",
    "numberOfRequests": {
        "total": "6",
        "ok": "0",
        "ko": "6"
    },
    "minResponseTime": {
        "total": "0",
        "ok": "-",
        "ko": "0"
    },
    "maxResponseTime": {
        "total": "1",
        "ok": "-",
        "ko": "1"
    },
    "meanResponseTime": {
        "total": "1",
        "ok": "-",
        "ko": "1"
    },
    "standardDeviation": {
        "total": "1",
        "ok": "-",
        "ko": "1"
    },
    "percentiles1": {
        "total": "1",
        "ok": "-",
        "ko": "1"
    },
    "percentiles2": {
        "total": "1",
        "ok": "-",
        "ko": "1"
    },
    "percentiles3": {
        "total": "1",
        "ok": "-",
        "ko": "1"
    },
    "percentiles4": {
        "total": "1",
        "ok": "-",
        "ko": "1"
    },
    "group1": {
    "name": "t < 800 ms",
    "htmlName": "t < 800 ms",
    "count": 0,
    "percentage": 0.0
},
    "group2": {
    "name": "800 ms <= t < 1200 ms",
    "htmlName": "t >= 800 ms <br> t < 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group3": {
    "name": "t >= 1200 ms",
    "htmlName": "t >= 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group4": {
    "name": "failed",
    "htmlName": "failed",
    "count": 6,
    "percentage": 100.0
},
    "meanNumberOfRequestsPerSecond": {
        "total": "1.5",
        "ok": "-",
        "ko": "1.5"
    }
}
    }
}

     },"req_get--ok-642241121": {
        type: "REQUEST",
        name: "GET /ok",
path: "outer / GET /ok",
pathFormatted: "req_outer---get--ok-2130245131",
stats: {
    "name": "GET /ok",
    "numberOfRequests": {
        "total": "66",
        "ok": "66",
        "ko": "0"
    },
    "minResponseTime": {
        "total": "0",
        "ok": "0",
        "ko": "-"
    },
    "maxResponseTime": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "meanResponseTime": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "standardDeviation": {
        "total": "0",
        "ok": "0",
        "ko": "-"
    },
    "percentiles1": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "percentiles2": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "percentiles3": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "percentiles4": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "group1": {
    "name": "t < 800 ms",
    "htmlName": "t < 800 ms",
    "count": 66,
    "percentage": 100.0
},
    "group2": {
    "name": "800 ms <= t < 1200 ms",
    "htmlName": "t >= 800 ms <br> t < 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group3": {
    "name": "t >= 1200 ms",
    "htmlName": "t >= 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group4": {
    "name": "failed",
    "htmlName": "failed",
    "count": 0,
    "percentage": 0.0
},
    "meanNumberOfRequestsPerSecond": {
        "total": "16.5",
        "ok": "16.5",
        "ko": "-"
    }
}
    }
}

     },"req_get--ok-642241121": {
        type: "REQUEST",
        name: "GET /ok",
path: "GET /ok",
pathFormatted: "req_get--ok-642241121",
stats: {
    "name": "GET /ok",
    "numberOfRequests": {
        "total": "6",
        "ok": "6",
        "ko": "0"
    },
    "minResponseTime": {
        "total": "3",
        "ok": "3",
        "ko": "-"
    },
    "maxResponseTime": {
        "total": "7",
        "ok": "7",
        "ko": "-"
    },
    "meanResponseTime": {
        "total": "6",
        "ok": "6",
        "ko": "-"
    },
    "standardDeviation": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "percentiles1": {
        "total": "6",
        "ok": "6",
        "ko": "-"
    },
    "percentiles2": {
        "total": "6",
        "ok": "6",
        "ko": "-"
    },
    "percentiles3": {
        "total": "7",
        "ok": "7",
        "ko": "-"
    },
    "percentiles4": {
        "total": "7",
        "ok": "7",
        "ko": "-"
    },
    "group1": {
    "name": "t < 800 ms",
    "htmlName": "t < 800 ms",
    "count": 6,
    "percentage": 100.0
},
    "group2": {
    "name": "800 ms <= t < 1200 ms",
    "htmlName": "t >= 800 ms <br> t < 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group3": {
    "name": "t >= 1200 ms",
    "htmlName": "t >= 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group4": {
    "name": "failed",
    "htmlName": "failed",
    "count": 0,
    "percentage": 0.0
},
    "meanNumberOfRequestsPerSecond": {
        "total": "1.5",
        "ok": "1.5",
        "ko": "-"
    }
}
    },"req_----------ok-569551681": {
        type: "REQUEST",
        name: "Проверка /ok",
path: "Проверка /ok",
pathFormatted: "req_----------ok-569551681",
stats: {
    "name": "Проверка /ok",
    "numberOfRequests": {
        "total": "6",
        "ok": "6",
        "ko": "0"
    },
    "minResponseTime": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "maxResponseTime": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "meanResponseTime": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "standardDeviation": {
        "total": "0",
        "ok": "0",
        "ko": "-"
    },
    "percentiles1": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "percentiles2": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "percentiles3": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "percentiles4": {
        "total": "1",
        "ok": "1",
        "ko": "-"
    },
    "group1": {
    "name": "t < 800 ms",
    "htmlName": "t < 800 ms",
    "count": 6,
    "percentage": 100.0
},
    "group2": {
    "name": "800 ms <= t < 1200 ms",
    "htmlName": "t >= 800 ms <br> t < 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group3": {
    "name": "t >= 1200 ms",
    "htmlName": "t >= 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group4": {
    "name": "failed",
    "htmlName": "failed",
    "count": 0,
    "percentage": 0.0
},
    "meanNumberOfRequestsPerSecond": {
        "total": "1.5",
        "ok": "1.5",
        "ko": "-"
    }
}
    },"req_connect-refused-472100250": {
        type: "REQUEST",
        name: "connect refused",
path: "connect refused",
pathFormatted: "req_connect-refused-472100250",
stats: {
    "name": "connect refused",
    "numberOfRequests": {
        "total": "6",
        "ok": "0",
        "ko": "6"
    },
    "minResponseTime": {
        "total": "1",
        "ok": "-",
        "ko": "1"
    },
    "maxResponseTime": {
        "total": "2",
        "ok": "-",
        "ko": "2"
    },
    "meanResponseTime": {
        "total": "2",
        "ok": "-",
        "ko": "2"
    },
    "standardDeviation": {
        "total": "0",
        "ok": "-",
        "ko": "0"
    },
    "percentiles1": {
        "total": "2",
        "ok": "-",
        "ko": "2"
    },
    "percentiles2": {
        "total": "2",
        "ok": "-",
        "ko": "2"
    },
    "percentiles3": {
        "total": "2",
        "ok": "-",
        "ko": "2"
    },
    "percentiles4": {
        "total": "2",
        "ok": "-",
        "ko": "2"
    },
    "group1": {
    "name": "t < 800 ms",
    "htmlName": "t < 800 ms",
    "count": 0,
    "percentage": 0.0
},
    "group2": {
    "name": "800 ms <= t < 1200 ms",
    "htmlName": "t >= 800 ms <br> t < 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group3": {
    "name": "t >= 1200 ms",
    "htmlName": "t >= 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group4": {
    "name": "failed",
    "htmlName": "failed",
    "count": 6,
    "percentage": 100.0
},
    "meanNumberOfRequestsPerSecond": {
        "total": "1.5",
        "ok": "-",
        "ko": "1.5"
    }
}
    },"req_unknown-host--1377806402": {
        type: "REQUEST",
        name: "unknown host",
path: "unknown host",
pathFormatted: "req_unknown-host--1377806402",
stats: {
    "name": "unknown host",
    "numberOfRequests": {
        "total": "6",
        "ok": "0",
        "ko": "6"
    },
    "minResponseTime": {
        "total": "0",
        "ok": "-",
        "ko": "0"
    },
    "maxResponseTime": {
        "total": "4",
        "ok": "-",
        "ko": "4"
    },
    "meanResponseTime": {
        "total": "2",
        "ok": "-",
        "ko": "2"
    },
    "standardDeviation": {
        "total": "2",
        "ok": "-",
        "ko": "2"
    },
    "percentiles1": {
        "total": "4",
        "ok": "-",
        "ko": "4"
    },
    "percentiles2": {
        "total": "4",
        "ok": "-",
        "ko": "4"
    },
    "percentiles3": {
        "total": "4",
        "ok": "-",
        "ko": "4"
    },
    "percentiles4": {
        "total": "4",
        "ok": "-",
        "ko": "4"
    },
    "group1": {
    "name": "t < 800 ms",
    "htmlName": "t < 800 ms",
    "count": 0,
    "percentage": 0.0
},
    "group2": {
    "name": "800 ms <= t < 1200 ms",
    "htmlName": "t >= 800 ms <br> t < 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group3": {
    "name": "t >= 1200 ms",
    "htmlName": "t >= 1200 ms",
    "count": 0,
    "percentage": 0.0
},
    "group4": {
    "name": "failed",
    "htmlName": "failed",
    "count": 6,
    "percentage": 100.0
},
    "meanNumberOfRequestsPerSecond": {
        "total": "1.5",
        "ok": "-",
        "ko": "1.5"
    }
}
    }
}

}

function fillStats(stat){
    $("#numberOfRequests").append(stat.numberOfRequests.total);
    $("#numberOfRequestsOK").append(stat.numberOfRequests.ok);
    $("#numberOfRequestsKO").append(stat.numberOfRequests.ko);

    $("#minResponseTime").append(stat.minResponseTime.total);
    $("#minResponseTimeOK").append(stat.minResponseTime.ok);
    $("#minResponseTimeKO").append(stat.minResponseTime.ko);

    $("#maxResponseTime").append(stat.maxResponseTime.total);
    $("#maxResponseTimeOK").append(stat.maxResponseTime.ok);
    $("#maxResponseTimeKO").append(stat.maxResponseTime.ko);

    $("#meanResponseTime").append(stat.meanResponseTime.total);
    $("#meanResponseTimeOK").append(stat.meanResponseTime.ok);
    $("#meanResponseTimeKO").append(stat.meanResponseTime.ko);

    $("#standardDeviation").append(stat.standardDeviation.total);
    $("#standardDeviationOK").append(stat.standardDeviation.ok);
    $("#standardDeviationKO").append(stat.standardDeviation.ko);

    $("#percentiles1").append(stat.percentiles1.total);
    $("#percentiles1OK").append(stat.percentiles1.ok);
    $("#percentiles1KO").append(stat.percentiles1.ko);

    $("#percentiles2").append(stat.percentiles2.total);
    $("#percentiles2OK").append(stat.percentiles2.ok);
    $("#percentiles2KO").append(stat.percentiles2.ko);

    $("#percentiles3").append(stat.percentiles3.total);
    $("#percentiles3OK").append(stat.percentiles3.ok);
    $("#percentiles3KO").append(stat.percentiles3.ko);

    $("#percentiles4").append(stat.percentiles4.total);
    $("#percentiles4OK").append(stat.percentiles4.ok);
    $("#percentiles4KO").append(stat.percentiles4.ko);

    $("#meanNumberOfRequestsPerSecond").append(stat.meanNumberOfRequestsPerSecond.total);
    $("#meanNumberOfRequestsPerSecondOK").append(stat.meanNumberOfRequestsPerSecond.ok);
    $("#meanNumberOfRequestsPerSecondKO").append(stat.meanNumberOfRequestsPerSecond.ko);
}
