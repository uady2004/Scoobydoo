import 'package:flutter/material.dart';

// ignore_for_file: deprecated_member_use
typedef DashboardScreen = CreatorDashboardScreen;

class CreatorDashboardScreen extends StatelessWidget {
  const CreatorDashboardScreen({super.key});

  @override
  Widget build(BuildContext context) {

    return Scaffold(

      appBar: AppBar(
        title: const Text(
          "Creator Dashboard",
        ),
      ),

      body: Padding(

        padding: const EdgeInsets.all(16),

        child: Column(

          children: [

            Card(

              child: ListTile(

                title:
                const Text("Total Views"),

                subtitle:
                const Text("1,250,000"),

                trailing:
                const Icon(Icons.bar_chart),

              ),

            ),

            Card(

              child: ListTile(

                title:
                const Text("Followers"),

                subtitle:
                const Text("152,000"),

                trailing:
                const Icon(Icons.people),

              ),

            ),

            Card(

              child: ListTile(

                title:
                const Text("Revenue"),

                subtitle:
                const Text("\$4,250"),

                trailing:
                const Icon(Icons.attach_money),

              ),

            ),

          ],

        ),

      ),

    );
  }
}