import 'package:flutter/material.dart';

class FeedProvider
    extends ChangeNotifier {

  List<String> videos = [];

  void loadFeed() {

    videos = [

      "Video 1",
      "Video 2",
      "Video 3",
      "Video 4",

    ];

    notifyListeners();
  }
}