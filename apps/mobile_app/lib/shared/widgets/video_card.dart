import 'package:flutter/material.dart';

class VideoCard extends StatelessWidget{

final String title;

const VideoCard({

super.key,

required this.title

});

@override
Widget build(BuildContext context){

return Container(

height:

MediaQuery.of(context).size.height,

color:Colors.black,

child:

Stack(

children:[

Center(

child:Text(

title,

style:

const TextStyle(

fontSize:35,

color:Colors.white

),

),

),

Positioned(

right:20,

bottom:100,

child:

Column(

children:[

IconButton(

onPressed:(){},

icon:

const Icon(

Icons.favorite,

color:Colors.red,

size:40,

),

),

IconButton(

onPressed:(){},

icon:

const Icon(

Icons.comment,

size:40,
),
),
],
)
)
],
)
);
}
}