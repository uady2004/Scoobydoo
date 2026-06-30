import 'package:flutter/material.dart';

class ReportScreen extends StatelessWidget {
  const ReportScreen({
    super.key,
    required this.contentId,
    required this.contentType,
  });

  final String contentId;
  final String contentType;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        title: const Text('Report', style: TextStyle(color: Colors.white)),
        iconTheme: const IconThemeData(color: Colors.white),
      ),
      body: Center(
        child: Text('Report $contentType $contentId — coming soon',
            style: const TextStyle(color: Colors.white54)),
      ),
    );
  }
}
