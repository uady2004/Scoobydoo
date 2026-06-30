import 'package:flutter/material.dart';

class BottomNav extends StatelessWidget {

  final int currentIndex;
  final Function(int) onTap;

  const BottomNav({
    super.key,
    required this.currentIndex,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {

    return BottomNavigationBar(

      currentIndex: currentIndex,

      onTap: onTap,

      backgroundColor: Colors.black,

      selectedItemColor: Colors.white,

      unselectedItemColor: Colors.grey,

      type: BottomNavigationBarType.fixed,

      items: const [

        BottomNavigationBarItem(
          icon: Icon(Icons.home),
          label: "Home",
        ),

        BottomNavigationBarItem(
          icon: Icon(Icons.search),
          label: "Search",
        ),

        BottomNavigationBarItem(
          icon: Icon(Icons.add_box),
          label: "Upload",
        ),

        BottomNavigationBarItem(
          icon: Icon(Icons.message),
          label: "Inbox",
        ),

        BottomNavigationBarItem(
          icon: Icon(Icons.person),
          label: "Profile",
        ),
      ],
    );
  }
}