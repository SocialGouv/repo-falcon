package com.example;

import java.util.List;
import static java.util.Collections.emptyList;

public class App {
  public static String run(String name) {
    List<String> xs = emptyList();
    return "hi " + name + " (" + xs.size() + ")";
  }
}

